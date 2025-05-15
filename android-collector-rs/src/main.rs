pub mod cmd;
pub mod cmd_help;
pub mod files;
pub mod helper;
pub mod process;
pub mod yara;

fn main() {
    env_logger::init();

    let args = cmd::cli().get_matches_from(wild::args());

    let _ = match args.subcommand() {
        Some(("yara", args)) => yara::exec(args),
        Some(("find", args)) => files::exec_find(args),
        Some(("ps", args)) => process::exec(args),
        _ => unreachable!(),
    };
}
