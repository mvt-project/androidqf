use clap::{command, crate_authors, Command};

use crate::files;
use crate::process;
use crate::yara;

pub fn command(name: &'static str) -> Command {
    Command::new(name).help_template(
        r#"{about-with-newline}
{usage-heading}
  {usage}

{all-args}
"#,
    )
}

pub fn cli() -> Command {
    command!()
        .author(crate_authors!("\n")) // requires `cargo` feature
        .arg_required_else_help(true)
        .subcommand_required(true)
        .subcommands(vec![
            yara::yara_cmds(),
            files::files_cmds(),
            process::process_cmds(),
        ])
}
