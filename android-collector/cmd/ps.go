package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tklauser/go-sysconf"
)

type ProcessInfo struct {
	Pid             int      `json:"pid"`
	Uid             int      `json:"uid"`
	Ppid            int      `json:"ppid"`
	Pgroup          int      `json:"pgroup"`
	Psid            int      `json:"psid"`
	Filename        string   `json:"filename"`
	Priority        int      `json:"priority"`
	State           string   `json:"state"`
	UserTime        int      `json:"user_time"`
	KernelTime      int      `json:"kernel_time"`
	Path            string   `json:"path"`
	Context         string   `json:"context"`
	PreviousContext string   `json:"previous_context"`
	CommandLine     []string `json:"command_line"`
	Env             []string `json:"env"`
	Cwd             string   `json:"cwd"`
}

func init() {
	rootCmd.AddCommand(processCmd)
}

var processCmd = &cobra.Command{
	Use:   "ps",
	Short: "List processes running on the phone",
	Long:  `List processes running on the phone.`,
	Run:   ps,
}

func conv(in []byte) string {
	return string(bytes.ReplaceAll(bytes.Trim(in, "\x00\n"), []byte("\x00"), []byte(" ")))
}

func (p *ProcessInfo) readStat() error {
	stat, err := os.Open(filepath.Join("/proc/", fmt.Sprint(p.Pid), "stat"))
	if err != nil {
		return err
	}

	_, err = fmt.Fscanf(stat,
		"%d %s %c %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		new(int),
		&p.Filename,
		&p.State,
		&p.Ppid,
		&p.Pgroup,
		&p.Psid,
		new(int),
		new(int),
		new(int),
		new(int),
		new(int),
		new(int),
		new(int),
		new(int),
		&p.UserTime,
		new(int),
		&p.KernelTime,
		new(int),
		new(int),
		&p.Priority,
	)
	if err != nil {
		return err
	}
	return nil
}

func (p *ProcessInfo) readCmdline() error {
	cmdlinePath := filepath.Join("/proc/", fmt.Sprint(p.Pid), "cmdline")
	cmdline, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return err
	}

	args := strings.Split(string(cmdline), "\x00")
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}

		p.CommandLine = append(p.CommandLine, arg)
	}

	return nil
}

func (p *ProcessInfo) readContext() error {
	// SELinux context
	dataBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/attr/current", p.Pid))
	if err == nil {
		p.Context = conv(dataBytes)
	}

	dataBytes, err = os.ReadFile(fmt.Sprintf("/proc/%d/attr/prev", p.Pid))
	if err == nil {
		p.PreviousContext = conv(dataBytes)
	}
	return nil
}

func (p *ProcessInfo) readEnv() error {
	environPath := filepath.Join("/proc/", fmt.Sprint(p.Pid), "environ")
	environ, err := os.ReadFile(environPath)
	if err != nil {
		return err
	}

	envs := strings.Split(string(environ), "\x00")
	for _, env := range envs {
		env = strings.TrimSpace(env)
		if env == "" {
			continue
		}

		p.Env = append(p.Env, env)
	}

	return nil
}

func (p *ProcessInfo) readCwd() error {
	cwdPath := filepath.Join("/proc/", fmt.Sprint(p.Pid), "cwd")
	cwd, err := os.Readlink(cwdPath)
	if err != nil {
		return err
	}
	p.Cwd = cwd
	return nil
}

// Execute the command
func ps(cmd *cobra.Command, args []string) {
	fh, err := os.Open("/proc")
	if err != nil {
		log.Fatal(err)
	}
	defer fh.Close()

	files, err := fh.ReadDir(0)
	if err != nil {
		log.Fatal(err)
	}

	var processes []ProcessInfo
	clktck, _ := sysconf.Sysconf(sysconf.SC_CLK_TCK)

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(file.Name())
		if err != nil {
			continue
		}

		var new_process ProcessInfo
		new_process.Pid = pid

		err = new_process.readStat()
		if err != nil {
			new_process.UserTime = new_process.UserTime / int(clktck)
			new_process.KernelTime = new_process.KernelTime / int(clktck)
		}
		new_process.readCmdline()
		new_process.readContext()
		new_process.readEnv()
		new_process.readCwd()

		processes = append(processes, new_process)
	}
	jsonData, err := json.Marshal(&processes)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonData))
}
