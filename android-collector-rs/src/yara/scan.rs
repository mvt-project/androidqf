use jwalk_meta::WalkDirGeneric;
use std::fs;

use flume::{unbounded, Receiver, Sender};
use std::fs::canonicalize;
use std::fs::Metadata;
use std::io::Error;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Instant;

use yara_x::{Patterns, Rule, Rules, Scanner};

use crate::helper::filter::{create_filter, filter_children, Filter};
use crate::helper::options::Options;
use crate::helper::ErrorsType;
use crate::yara::scan_result::{PatternJson, RuleJson, ScanResult, ScanResults, YaraEntry};

fn rules_to_json(scan_results: &mut dyn ExactSizeIterator<Item = Rule>) -> Vec<RuleJson> {
    scan_results
        .map(move |rule| RuleJson {
            identifier: rule.identifier().to_string(),
            namespace: Some(rule.namespace().to_string()),
            meta: Some(rule.metadata().into_json()),
            tags: Some(
                rule.tags()
                    .map(|t| t.identifier().to_string())
                    .collect::<Vec<_>>(),
            ),
            strings: Some(patterns_to_json(rule.patterns(), 1024)),
        })
        .collect()
}

fn patterns_to_json(patterns: Patterns<'_, '_>, string_limit: usize) -> Vec<PatternJson> {
    patterns
        .flat_map(|pattern| {
            let identifier = pattern.identifier();

            pattern.matches().map(|pattern_match| {
                let match_range = pattern_match.range();
                let match_data = pattern_match.data();

                let more_bytes_message = match match_data.len().saturating_sub(string_limit) {
                    0 => None,
                    n => Some(format!(" ... {} more bytes", n)),
                };

                let string = match_data
                    .iter()
                    .take(string_limit)
                    .flat_map(|char| char.escape_ascii())
                    .map(|c| c as char)
                    .chain(more_bytes_message.iter().flat_map(|msg| msg.chars()))
                    .collect::<String>();

                PatternJson {
                    identifier: identifier.to_owned(),
                    offset: match_range.start,
                    r#match: string,
                    xor_key: pattern_match.xor_key(),
                    plaintext: pattern_match.xor_key().map(|xor_key| {
                        match_data
                            .iter()
                            .take(string_limit)
                            .map(|char| char ^ xor_key)
                            .flat_map(|char| char.escape_ascii())
                            .map(|char| char as char)
                            .collect()
                    }),
                }
            })
        })
        .collect()
}

pub fn get_root_path_len(root_path: &Path) -> usize {
    let root_path = root_path.to_str().unwrap();
    let mut root_path_len = root_path.len();
    if !root_path.ends_with('/') {
        root_path_len += 1;
    }
    root_path_len
}

#[inline]
fn create_entry(
    rules: Arc<Mutex<Rules>>,
    dir_entry: &jwalk_meta::DirEntry<((), Option<Result<Metadata, Error>>)>,
) -> ScanResult {
    let file_type = dir_entry.file_type;

    let mut matched_count = 0;
    let mut rules_result = Vec::new();

    let path = String::from(dir_entry.path().to_str().unwrap());

    if file_type.is_file() {
        let rules = rules.lock().unwrap();

        let mut scanner = Scanner::new(&rules);

        let _ = match scanner.scan_file(path.clone()) {
            Err(_) => {}
            Ok(scan_result) => {
                let mut wanted_rules = scan_result.matching_rules();
                let rules = rules_to_json(&mut wanted_rules);
                matched_count = wanted_rules.len();

                rules_result = rules.clone();
            }
        };
    }

    let entry: ScanResult = ScanResult::YaraEntry(YaraEntry {
        path,
        count: matched_count,
        rules: rules_result,
    });
    entry
}

fn entries_thread(
    options: Options,
    rule_path: String,
    filter: Option<Filter>,
    tx: Sender<ScanResult>,
    stop: Arc<AtomicBool>,
) {
    let file_yara = fs::File::open(rule_path.clone()).unwrap();

    let rules = Rules::deserialize_from(file_yara).unwrap();

    let root_path_len = get_root_path_len(&options.root_path);

    let dir_entry: jwalk_meta::DirEntry<((), Option<Result<Metadata, Error>>)> =
        jwalk_meta::DirEntry::from_path(
            0,
            &options.root_path,
            true,
            true,
            options.follow_links,
            Arc::new(Vec::new()),
        )
        .unwrap();

    let rules_m = Arc::new(Mutex::new(rules));

    if !dir_entry.file_type.is_dir() {
        let _ = tx.send(create_entry(rules_m.clone(), &dir_entry));
        return;
    }

    let max_file_cnt = options.max_file_cnt;
    let mut file_cnt = 0;

    for result in WalkDirGeneric::new(&options.root_path)
        .skip_hidden(options.skip_hidden)
        .follow_links(options.follow_links)
        .sort(options.sorted)
        .max_depth(options.max_depth)
        .read_metadata(true)
        .process_read_dir(move |_, root_dir, _, children| {
            if let Some(root_dir) = root_dir.to_str() {
                if root_dir.len() + 1 < root_path_len {
                    return;
                }
            } else {
                return;
            }
            filter_children(children, &filter);
            children.iter_mut().for_each(|dir_entry_result| {
                if let Ok(dir_entry) = dir_entry_result {
                    if tx.send(create_entry(rules_m.clone(), dir_entry)).is_err() {
                        return;
                    }
                }
            });
        })
    {
        if stop.load(Ordering::Relaxed) {
            break;
        }
        if let Ok(dir_entry) = result {
            if !dir_entry.file_type.is_dir() {
                file_cnt += 1;
                if max_file_cnt > 0 && file_cnt > max_file_cnt {
                    break;
                }
            }
        }
    }
}

#[derive(Debug)]
pub struct Scan {
    // Options
    options: Options,
    rule_path: String,
    store: bool,
    // Results
    entries: ScanResults,
    duration: Arc<Mutex<f64>>,
    finished: Arc<AtomicBool>,
    // Internal
    thr: Option<thread::JoinHandle<()>>,
    stop: Arc<AtomicBool>,
    rx: Option<Receiver<ScanResult>>,
}

impl Scan {
    pub fn new<P: AsRef<Path>>(
        root_path: P,
        rule_path: P,
        store: Option<bool>,
    ) -> Result<Self, Error> {
        let rule_path_str = String::from(rule_path.as_ref().to_str().unwrap());

        Ok(Scan {
            options: Options {
                root_path: canonicalize(root_path.as_ref().to_str().unwrap())?,
                sorted: false,
                skip_hidden: false,
                max_depth: usize::MAX,
                max_file_cnt: usize::MAX,
                dir_include: None,
                dir_exclude: None,
                file_include: None,
                file_exclude: None,
                case_sensitive: false,
                follow_links: false,
            },
            rule_path: rule_path_str.clone(),
            store: store.unwrap_or(true),
            entries: ScanResults::new(),
            duration: Arc::new(Mutex::new(0.0)),
            finished: Arc::new(AtomicBool::new(false)),
            thr: None,
            stop: Arc::new(AtomicBool::new(false)),
            rx: None,
        })
    }

    /// Return results in sorted order.
    pub fn sorted(mut self, sorted: bool) -> Self {
        self.options.sorted = sorted;
        self
    }

    /// Skip hidden entries. Enabled by default.
    pub fn skip_hidden(mut self, skip_hidden: bool) -> Self {
        self.options.skip_hidden = skip_hidden;
        self
    }

    /// Set the maximum depth of entries yield by the iterator.
    ///
    /// The smallest depth is `0` and always corresponds to the path given
    /// to the `new` function on this type. Its direct descendents have depth
    /// `1`, and their descendents have depth `2`, and so on.
    ///
    /// Note that this will not simply filter the entries of the iterator, but
    /// it will actually avoid descending into directories when the depth is
    /// exceeded.
    pub fn max_depth(mut self, depth: usize) -> Self {
        self.options.max_depth = match depth {
            0 => usize::MAX,
            _ => depth,
        };
        self
    }

    /// Set maximum number of files to collect
    pub fn max_file_cnt(mut self, max_file_cnt: usize) -> Self {
        self.options.max_file_cnt = match max_file_cnt {
            0 => usize::MAX,
            _ => max_file_cnt,
        };
        self
    }

    /// Set directory include filter
    pub fn dir_include(mut self, dir_include: Option<Vec<String>>) -> Self {
        self.options.dir_include = dir_include;
        self
    }

    /// Set directory exclude filter
    pub fn dir_exclude(mut self, dir_exclude: Option<Vec<String>>) -> Self {
        self.options.dir_exclude = dir_exclude;
        self
    }

    /// Set file include filter
    pub fn file_include(mut self, file_include: Option<Vec<String>>) -> Self {
        self.options.file_include = file_include;
        self
    }

    /// Set file exclude filter
    pub fn file_exclude(mut self, file_exclude: Option<Vec<String>>) -> Self {
        self.options.file_exclude = file_exclude;
        self
    }

    /// Set case sensitive filename filtering
    pub fn case_sensitive(mut self, case_sensitive: bool) -> Self {
        self.options.case_sensitive = case_sensitive;
        self
    }

    /// Set follow symlinks
    pub fn follow_links(mut self, follow_links: bool) -> Self {
        self.options.follow_links = follow_links;
        self
    }

    pub fn clear(&mut self) {
        self.entries.clear();
        *self.duration.lock().unwrap() = 0.0;
    }

    pub fn start(&mut self) -> Result<(), Error> {
        if self.busy() {
            return Err(Error::other("Busy"));
        }

        self.clear();
        let options = self.options.clone();
        let filter = create_filter(&options)?;
        let (tx, rx) = unbounded();
        self.rx = Some(rx);
        self.stop.store(false, Ordering::Relaxed);
        let stop = self.stop.clone();
        let duration = self.duration.clone();
        let finished = self.finished.clone();

        let rule_path = self.rule_path.clone();

        self.thr = Some(thread::spawn(move || {
            let start_time = Instant::now();
            entries_thread(options, rule_path, filter, tx, stop);
            *duration.lock().unwrap() = start_time.elapsed().as_secs_f64();
            finished.store(true, Ordering::Relaxed);
        }));
        Ok(())
    }

    pub fn join(&mut self) -> bool {
        if let Some(thr) = self.thr.take() {
            if let Err(_e) = thr.join() {
                return false;
            }
            return true;
        }
        false
    }

    pub fn stop(&mut self) -> bool {
        if let Some(thr) = self.thr.take() {
            self.stop.store(true, Ordering::Relaxed);
            if let Err(_e) = thr.join() {
                return false;
            }
            return true;
        }
        false
    }

    pub fn collect(&mut self) -> Result<ScanResults, Error> {
        if !self.finished() {
            if !self.busy() {
                self.start()?;
            }
            self.join();
        }
        Ok(self.results(true))
    }

    pub fn has_results(&mut self, only_new: bool) -> bool {
        if let Some(ref rx) = self.rx {
            if !rx.is_empty() {
                return true;
            }
        }
        if only_new {
            return false;
        }
        !self.entries.is_empty()
    }

    pub fn results_cnt(&mut self, only_new: bool) -> usize {
        if let Some(ref rx) = self.rx {
            if only_new {
                rx.len()
            } else {
                self.entries.len() + rx.len()
            }
        } else if only_new {
            0
        } else {
            self.entries.len()
        }
    }

    pub fn results(&mut self, only_new: bool) -> ScanResults {
        let mut results = ScanResults::new();
        if let Some(ref rx) = self.rx {
            while let Ok(entry) = rx.try_recv() {
                if let ScanResult::Error(e) = entry {
                    results.errors.push(e);
                } else {
                    results.results.push(entry);
                }
            }
        }
        if self.store {
            self.entries.extend(&results);
        }
        if !only_new && self.store {
            return self.entries.clone();
        }
        results
    }

    pub fn has_entries(&mut self, only_new: bool) -> bool {
        if let Some(ref rx) = self.rx {
            if !rx.is_empty() {
                return true;
            }
        }
        if only_new {
            return false;
        }
        !self.entries.is_empty()
    }

    pub fn entries_cnt(&mut self, only_new: bool) -> usize {
        if let Some(ref rx) = self.rx {
            if only_new {
                return rx.len();
            }
            self.entries.len() + rx.len()
        } else {
            self.entries.len()
        }
    }

    pub fn entries(&mut self, only_new: bool) -> Vec<ScanResult> {
        self.results(only_new).results
    }

    pub fn has_errors(&mut self) -> bool {
        !self.entries.errors.is_empty()
    }

    pub fn errors_cnt(&mut self) -> usize {
        self.entries.errors.len()
    }

    pub fn errors(&mut self, only_new: bool) -> ErrorsType {
        self.results(only_new).errors
    }

    pub fn to_json(&self) -> serde_json::Result<String> {
        self.entries.to_json()
    }

    pub fn duration(&mut self) -> f64 {
        *self.duration.lock().unwrap()
    }

    pub fn finished(&mut self) -> bool {
        self.finished.load(Ordering::Relaxed)
    }

    pub fn busy(&self) -> bool {
        if let Some(ref thr) = self.thr {
            !thr.is_finished()
        } else {
            false
        }
    }

    pub fn options(&self) -> Options {
        self.options.clone()
    }
}
