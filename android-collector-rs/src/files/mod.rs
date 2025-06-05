pub mod scandir;
pub mod scandir_result;

use clap::{arg, value_parser, ArgMatches, Command};
use log::info;

use crate::cmd;
use crate::cmd_help;

use scandir::Scandir;

pub fn files_cmds() -> Command {
    cmd::command("find")
        .about("List all files from a specific path with their attributes")
        .long_about(cmd_help::FILES_FIND_LONG_HELP)
        .arg(
            arg!(-p --"path" <PATH>)
                .help("Scan the specific PATH")
                .value_parser(value_parser!(String)),
        )
}

pub fn exec_find(args: &ArgMatches) -> anyhow::Result<()> {
    info!("EXEC FIND");

    let scan = Scandir::new(args.get_one::<String>("path").unwrap(), None)?
        .dir_exclude(Some(vec!["/proc/**".to_string(), "/sys/**".to_string()]))
        .max_depth(5)
        .follow_links(false)
        .collect()?;

    // Get the scans and convert to something compatible with AndroidQF
    for file in scan.results {
        println!("{:?}", file.to_json().unwrap());
    }
    Ok(())
}
