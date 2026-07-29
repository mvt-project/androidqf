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
	"syscall"

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
	UserTime        int64    `json:"user_time"`
	KernelTime      int64    `json:"kernel_time"`
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
	data, err := os.ReadFile(filepath.Join("/proc/", fmt.Sprint(p.Pid), "stat"))
	if err != nil {
		return err
	}

	return p.parseStat(string(data))
}

func (p *ProcessInfo) parseStat(stat string) error {
	open := strings.IndexByte(stat, '(')
	close := strings.LastIndex(stat, ")")
	if open < 0 || close <= open {
		return fmt.Errorf("malformed process stat")
	}

	pid, err := strconv.Atoi(strings.TrimSpace(stat[:open]))
	if err != nil {
		return fmt.Errorf("invalid process id: %w", err)
	}
	fields := strings.Fields(stat[close+1:])
	if len(fields) < 16 {
		return fmt.Errorf("malformed process stat: got %d fields after command", len(fields))
	}

	parseInt := func(index int, name string) (int, error) {
		value, err := strconv.Atoi(fields[index])
		if err != nil {
			return 0, fmt.Errorf("invalid %s: %w", name, err)
		}
		return value, nil
	}
	parseInt64 := func(index int, name string) (int64, error) {
		value, err := strconv.ParseInt(fields[index], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s: %w", name, err)
		}
		return value, nil
	}

	ppid, err := parseInt(1, "parent process id")
	if err != nil {
		return err
	}
	pgroup, err := parseInt(2, "process group")
	if err != nil {
		return err
	}
	psid, err := parseInt(3, "session id")
	if err != nil {
		return err
	}
	userTime, err := parseInt64(11, "user time")
	if err != nil {
		return err
	}
	kernelTime, err := parseInt64(12, "kernel time")
	if err != nil {
		return err
	}
	priority, err := parseInt(15, "priority")
	if err != nil {
		return err
	}

	p.Pid = pid
	p.Filename = stat[open+1 : close]
	p.State = fields[0]
	p.Ppid = ppid
	p.Pgroup = pgroup
	p.Psid = psid
	p.UserTime = userTime
	p.KernelTime = kernelTime
	p.Priority = priority
	return nil
}

func (p *ProcessInfo) readIdentityAndPath() {
	procPath := filepath.Join("/proc/", fmt.Sprint(p.Pid))
	if info, err := os.Stat(procPath); err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			p.Uid = int(stat.Uid)
		}
	}
	if executable, err := os.Readlink(filepath.Join(procPath, "exe")); err == nil {
		p.Path = executable
	}
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
	clktck, clockErr := sysconf.Sysconf(sysconf.SC_CLK_TCK)

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
		if err == nil && clockErr == nil && clktck > 0 {
			new_process.UserTime /= clktck
			new_process.KernelTime /= clktck
		}
		new_process.readIdentityAndPath()
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
