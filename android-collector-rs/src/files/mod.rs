pub mod scandir;
pub mod scandir_result;

use clap::{arg, value_parser, ArgAction, ArgMatches, Command};
use log::info;
use serde::{Deserialize, Serialize};

use crate::cmd;
use crate::cmd_help;

use scandir::Scandir;

pub fn files_find_cmd() -> Command {
    cmd::command("find")
        .about("List all files from a specific path with their attributes")
        .long_about(cmd_help::FILES_FIND_LONG_HELP)
        .arg(
            arg!(-p --"path" <PATH>)
                .help("Scan the provided directory or file path")
                .value_parser(value_parser!(String)),
        )
        .arg(
            arg!(-e - -"exclude_dir")
                .help("Exclude a directory from the paths")
                .value_parser(value_parser!(String))
                .action(ArgAction::Append),
        )
}

#[derive(Deserialize, Serialize, Debug, Clone)]
pub struct AndroidQFFileInfo {
    path: String,
    size: u64,
    mode: String,
    user_id: u32,
    user_name: String,
    group_id: u32,
    group_name: String,
    changed_time: i64,
    modified_time: i64,
    access_time: i64,
    error: String,
    context: String,
    sha1: String,
    sha256: String,
    sha512: String,
    md5: String,
}

pub fn exec_find(args: &ArgMatches) -> anyhow::Result<()> {
    info!("[collector][files][find]");

    let mut excluded_dirs = Vec::new();
    for dir in args
        .try_get_many::<String>("exclude_dir")
        .unwrap_or_default()
        .into_iter()
        .flatten()
    {
        excluded_dirs.push(dir.clone());
    }

    let scan = Scandir::new(args.get_one::<String>("path").unwrap(), None)?
        .dir_exclude(Some(excluded_dirs))
        .max_depth(5)
        .follow_links(false)
        .collect()?;

    // Get the scans and convert to struct compatible with AndroidQF, in order to keep compatibility
    for file in scan.results {
        let m = AndroidQFFileInfo {
            path: file.path().clone(),
            size: file.size(),
            mode: "".to_string(),
            user_id: file.uid(),
            user_name: "".to_string(),
            group_id: file.gid(),
            group_name: "".to_string(),
            changed_time: file.ctime() as i64,
            modified_time: file.mtime() as i64,
            access_time: file.atime() as i64,
            error: "".to_string(),
            context: "".to_string(),
            sha1: "".to_string(),
            sha256: file.digest().clone(),
            sha512: "".to_string(),
            md5: "".to_string(),
        };

        println!("{}", serde_json::to_string(&m).unwrap());
    }
    Ok(())
}
