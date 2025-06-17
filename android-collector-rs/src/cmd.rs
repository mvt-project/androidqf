use clap::{command, Command};

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
    command!().subcommand_required(true).subcommands(vec![
        files::files_find_cmd(),
        process::process_ps_cmd(),
        yara::yara_cmd(),
    ])
}
