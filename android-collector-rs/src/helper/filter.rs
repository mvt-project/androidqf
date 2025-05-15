use glob_sl::{MatchOptions, Pattern};
use std::fs::Metadata;
use std::io::{Error, ErrorKind};

use crate::helper::options::Options;

#[derive(Debug, Clone, PartialEq)]
pub struct Filter {
    pub dir_include: Vec<Pattern>,
    pub dir_exclude: Vec<Pattern>,
    pub file_include: Vec<Pattern>,
    pub file_exclude: Vec<Pattern>,
    pub options: Option<MatchOptions>,
}

pub fn create_filter(options: &Options) -> Result<Option<Filter>, Error> {
    let mut filter = Filter {
        dir_include: Vec::new(),
        dir_exclude: Vec::new(),
        file_include: Vec::new(),
        file_exclude: Vec::new(),
        options: match options.case_sensitive {
            true => None,
            false => Some(MatchOptions {
                case_sensitive: false,
                ..MatchOptions::new()
            }),
        },
    };
    if let Some(ref f) = options.dir_include {
        let f = &mut f
            .iter()
            .map(|s| Pattern::new(s))
            .collect::<Result<Vec<_>, glob_sl::PatternError>>();
        let f = match f {
            Ok(f) => f,
            Err(e) => {
                return Err(Error::new(
                    ErrorKind::InvalidInput,
                    format!("dir_include: {}", e),
                ));
            }
        };
        filter.dir_include.append(f);
    }
    if let Some(ref f) = options.dir_exclude {
        let f = &mut f
            .iter()
            .map(|s| Pattern::new(s))
            .collect::<Result<Vec<_>, glob_sl::PatternError>>();
        let f = match f {
            Ok(f) => f,
            Err(e) => {
                return Err(Error::new(
                    ErrorKind::InvalidInput,
                    format!("dir_exclude: {}", e),
                ));
            }
        };
        filter.dir_exclude.append(f);
    }
    if let Some(ref f) = options.file_include {
        let f = &mut f
            .iter()
            .map(|s| Pattern::new(s))
            .collect::<Result<Vec<_>, glob_sl::PatternError>>();
        let f = match f {
            Ok(f) => f,
            Err(e) => {
                return Err(Error::new(
                    ErrorKind::InvalidInput,
                    format!("file_include: {}", e),
                ));
            }
        };
        filter.file_include.append(f);
    }
    if let Some(ref f) = options.file_exclude {
        let f = &mut f
            .iter()
            .map(|s| Pattern::new(s))
            .collect::<Result<Vec<_>, glob_sl::PatternError>>();
        let f = match f {
            Ok(f) => f,
            Err(e) => {
                return Err(Error::new(
                    ErrorKind::InvalidInput,
                    format!("file_exclude: {}", e),
                ));
            }
        };
        filter.file_exclude.append(f);
    }
    if filter.dir_include.is_empty()
        && filter.dir_exclude.is_empty()
        && filter.file_include.is_empty()
        && filter.file_exclude.is_empty()
    {
        return Ok(None);
    }
    Ok(Some(filter))
}

#[inline]
pub fn filter_direntry(
    key: &str,
    filter: &Vec<Pattern>,
    options: Option<MatchOptions>,
    empty: bool,
) -> bool {
    if filter.is_empty() || key.is_empty() {
        return empty;
    }
    match options {
        Some(options) => {
            for f in filter {
                if f.as_str().ends_with("**") && !key.ends_with('/') {
                    // Workaround: glob currently has problems with "foo/**"
                    let mut key = String::from(key);
                    key.push('/');
                    if f.matches_with(&key, options) {
                        return true;
                    }
                }
                if f.matches_with(key, options) {
                    return true;
                }
            }
        }
        None => {
            for f in filter {
                if f.as_str().ends_with("**") && !key.ends_with('/') {
                    // Workaround: glob currently has problems with "foo/**"
                    let mut key = String::from(key);
                    key.push('/');
                    if f.matches(&key) {
                        return true;
                    }
                }
                if f.matches(key) {
                    return true;
                }
            }
        }
    }
    false
}

#[inline]
pub fn filter_dir(
    dir_entry: &jwalk_meta::DirEntry<((), Option<Result<Metadata, Error>>)>,
    filter_ref: &Filter,
) -> bool {
    let mut key = dir_entry.parent_path.to_path_buf();
    key.push(dir_entry.file_name.clone().into_string().unwrap());

    let key = key.to_str().unwrap();

    /*   let key = key
    .to_str()
    .unwrap()
    .get(root_path_len..)
    .unwrap_or("")
    .to_string();*/

    if filter_direntry(&key, &filter_ref.dir_exclude, filter_ref.options, false)
        || !filter_direntry(&key, &filter_ref.dir_include, filter_ref.options, true)
    {
        return false;
    }
    true
}

#[inline]
pub fn filter_children(
    children: &mut Vec<
        Result<jwalk_meta::DirEntry<((), Option<Result<Metadata, Error>>)>, jwalk_meta::Error>,
    >,
    filter: &Option<Filter>,
) {
    if let Some(filter_ref) = &filter {
        children.retain(|dir_entry_result| {
            dir_entry_result
                .as_ref()
                .map(|dir_entry| {
                    if dir_entry.file_type.is_dir() {
                        return filter_dir(dir_entry, filter_ref);
                    } else {
                        let options = filter_ref.options;
                        let key = match dir_entry.file_name.to_str() {
                            Some(s) => s,
                            None => {
                                return false;
                            }
                        };
                        if filter_direntry(key, &filter_ref.file_exclude, options, false)
                            || !filter_direntry(key, &filter_ref.file_include, options, true)
                        {
                            return false;
                        }
                    }
                    true
                })
                .unwrap_or(false)
        });
    }
}
