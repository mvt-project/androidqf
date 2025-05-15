use clap::{ArgMatches, Command};
use serde::{Deserialize, Serialize};

use log::info;

use crate::cmd;
use crate::cmd_help;

#[derive(Deserialize, Serialize, Debug, Clone)]
pub struct ProcessInfo {
    pid: i32,
    uid: u32,
    ppid: i32,
    pgroup: i32,
    psid: i32,
    filename: String,
    priority: i64,
    state: String,
    user_time: u64,
    kernel_time: u64,
    path: String,
    context: String,
    previous_context: String,
    command_line: Vec<String>,
    env: Vec<String>,
    cwd: String,
}

pub fn process_cmds() -> Command {
    cmd::command("ps")
        .about("Get process list information")
        .long_about(cmd_help::PROCESS_LONG_HELP)
}

pub fn exec(_args: &ArgMatches) -> anyhow::Result<()> {
    info!("EXEC PROCESS");

    let mut processes = Vec::new();

    for prc in procfs::process::all_processes().unwrap() {
        let prc = prc.unwrap();
        let uid = prc.uid().unwrap();
        let stat = prc.stat().unwrap();

        let mut env = Vec::new();

        if let Ok(prc_environ) = prc.environ() {
            for (key, value) in prc_environ.into_iter() {
                env.push(format!("{:?}={:?}", key, value));
            }
        }

        let mut exe = String::new();
        if let Ok(prc_exe) = prc.exe() {
            exe = prc_exe.display().to_string()
        }

        let mut cwd = String::new();
        if let Ok(prc_cwd) = prc.cwd() {
            cwd = prc_cwd.display().to_string()
        }

        let mut command_line = Vec::new();
        if let Ok(prc_cmdline) = prc.cmdline() {
            command_line = prc_cmdline.clone();
        }

        processes.push(ProcessInfo {
            pid: stat.pid,
            uid: uid,
            ppid: stat.ppid,
            pgroup: stat.pgrp,
            psid: stat.session,
            filename: stat.comm.clone(),
            priority: stat.priority,
            state: String::from(stat.state),
            user_time: stat.utime,
            kernel_time: stat.stime,
            path: exe,
            context: "".to_string(),
            previous_context: "".to_string(),
            command_line: command_line,
            env: env.clone(),
            cwd: cwd,
        })
    }

    println!("{:?}", serde_json::to_string(&processes));

    Ok(())
}
