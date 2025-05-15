pub mod scan;
pub mod scan_result;

use clap::{arg, value_parser, ArgMatches, Command};
use log::info;

use crate::cmd;
use crate::cmd_help;

use scan::Scan;

pub fn yara_cmds() -> Command {
    cmd::command("yara")
        .about("Scan a file or directory with Yara-x")
        .long_about(cmd_help::YARA_SCAN_LONG_HELP)
        .arg(
            arg!(-p --"path" <PATH>)
                .help("Scan the specific PATH")
                .value_parser(value_parser!(String)),
        )
        .arg(
            arg!(-r --"rule_path" <PATH>)
                .help("Use a specific rule PATH")
                .value_parser(value_parser!(String)),
        )
}

pub fn exec(args: &ArgMatches) -> anyhow::Result<()> {
    info!("EXEC SCAN");

    let scan = Scan::new(
        args.get_one::<String>("path").unwrap(),
        args.get_one::<String>("rule_path").unwrap(),
        None,
    )?
    .dir_exclude(Some(vec![
        "/proc/**".to_string(),
        "/sys/**".to_string(),
        "/system/**".to_string(),
    ]))
    .max_depth(5)
    .follow_links(false)
    .collect()?;

    println!("{:?}", scan);

    Ok(())
}
