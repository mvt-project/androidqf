use clap::{ArgMatches, Command};
use serde::{Deserialize, Serialize};

use log::info;

use crate::cmd;
use crate::cmd_help;

#[derive(Deserialize, Serialize, Debug, Clone)]
pub struct AndroidQFProcessInfo {
    pid: u32,
    uid: u32,
    ppid: u32,
    pgroup: u32,
    psid: u32,
    filename: String,
    priority: u32,
    state: String,
    user_time: u32,
    kernel_time: u32,
    path: String,
    context: String,
    previous_context: String,
    command_line: Vec<String>,
    env: Vec<String>,
    cwd: String,
}

pub fn process_ps_cmd() -> Command {
    cmd::command("ps")
        .about("Get process list information")
        .long_about(cmd_help::PROCESS_LONG_HELP)
}

pub fn exec(_args: &ArgMatches) -> anyhow::Result<()> {
    info!("[collector][process][ps]");

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

        processes.push(AndroidQFProcessInfo {
            pid: stat.pid as u32,
            uid,
            ppid: stat.ppid as u32,
            pgroup: stat.pgrp as u32,
            psid: stat.session as u32,
            filename: stat.comm.clone(),
            priority: stat.priority as u32,
            state: String::from(stat.state),
            user_time: stat.utime as u32,
            kernel_time: stat.stime as u32,
            path: exe,
            context: "".to_string(),
            previous_context: "".to_string(),
            command_line,
            env: env.clone(),
            cwd,
        })
    }

    println!("{}", serde_json::to_string(&processes).unwrap());

    Ok(())
}
