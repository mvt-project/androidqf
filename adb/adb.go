// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	saveSlice "github.com/botherder/go-savetime/slice"
	"github.com/mvt-project/androidqf/assets"
	"github.com/mvt-project/androidqf/log"
)

type ADB struct {
	ExePath string
	Serial  string
	ctxMu   sync.RWMutex
	ctx     context.Context
}

type DeviceInfo struct {
	Serial  string
	State   string
	Product string
	Model   string
	Device  string
}

var Client *ADB

// New returns a new ADB instance.
func New() (*ADB, error) {
	return NewWithContext(context.Background())
}

// NewWithContext returns a new ADB instance whose commands are canceled when
// ctx is canceled.
func NewWithContext(ctx context.Context) (*ADB, error) {
	adb := ADB{}
	adb.SetContext(ctx)
	err := adb.findExe()
	if err != nil {
		return nil, fmt.Errorf("failed to find a usable adb executable: %v",
			err)
	}
	log.Debugf("ADB found at path: %s", adb.ExePath)

	// Confirm that we can call "adb devices" without errors
	_, err = adb.Devices()
	if err != nil {
		_, _ = adb.KillServer()
		_ = assets.CleanAssets()
		return nil, err
	}
	return &adb, nil
}

// SetContext changes the context used by subsequent ADB commands.
func (a *ADB) SetContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.ctxMu.Lock()
	a.ctx = ctx
	a.ctxMu.Unlock()
}

func (a *ADB) command(args ...string) *exec.Cmd {
	a.ctxMu.RLock()
	ctx := a.ctx
	a.ctxMu.RUnlock()
	if ctx == nil {
		ctx = context.Background()
	}
	return exec.CommandContext(ctx, a.ExePath, args...)
}

func (a *ADB) SetSerial(serial string) (string, error) {
	devices, err := a.Devices()
	if err != nil {
		return "", err
	}

	serial = strings.TrimSpace(serial)
	if len(devices) == 0 {
		return "", fmt.Errorf("no devices detected over ADB")
	}

	if serial != "" {
		// Check that the serial match one of the devices
		// Can be replace with the go package slices in 1.21
		if !saveSlice.ContainsNoCase(devices, serial) {
			// Serial is not an existing device
			return "", fmt.Errorf("serial %s not found in the device list", serial)
		}
		a.Serial = serial
	} else {
		if len(devices) > 1 {
			return "", fmt.Errorf("multiple devices connected, please stop AndroidQF and provide a serial number")
		}
		a.Serial = devices[0]
	}
	return a.Serial, nil
}

// List existing devices
func (a *ADB) Devices() ([]string, error) {
	var devices []string
	out, err := a.command("devices").Output()
	if err != nil {
		return devices, fmt.Errorf("failed to use the adb executable: %v",
			err)
	}

	lines := strings.Split(string(out), "\n")
	for _, s := range lines[1:] {
		dev := strings.Split(s, "\t")
		if len(dev) == 2 {
			devices = append(devices, strings.TrimSpace(dev[0]))
			log.Debug("Found new device: ", dev[0])
		}
	}

	return devices, nil
}

func (a *ADB) DeviceInfos() ([]DeviceInfo, error) {
	var devices []DeviceInfo
	out, err := a.command("devices", "-l").Output()
	if err != nil {
		return devices, fmt.Errorf("failed to use the adb executable: %v",
			err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines[1:] {
		info, ok := parseDeviceInfoLine(line)
		if !ok {
			continue
		}
		devices = append(devices, info)
		log.Debug("Found new device: ", info.Serial)
	}

	return devices, nil
}

func parseDeviceInfoLine(line string) (DeviceInfo, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return DeviceInfo{}, false
	}

	info := DeviceInfo{
		Serial: fields[0],
		State:  fields[1],
	}
	for _, field := range fields[2:] {
		key, value, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}

		switch key {
		case "product":
			info.Product = value
		case "model":
			info.Model = value
		case "device":
			info.Device = value
		}
	}

	return info, true
}

// Run a command to the given phone using exec
// Returns string and/or error
func (a *ADB) Exec(args ...string) ([]byte, error) {
	if a.Serial == "" {
		return a.command(args...).Output()
	} else {
		var params []string
		params = append(params, "-s", a.Serial)
		params = append(params, args...)
		return a.command(params...).Output()
	}
}

// GetState returns the output of `adb get-state`.
// It is used to check whether a device is connected. If it is not, adb
// will exit with status 1.
func (a *ADB) GetState() (string, error) {
	log.Debug("Starting get-state")
	out, err := a.Exec("get-state")
	if err != nil {
		log.Debug("get-state failed")
		return "", err
	}

	log.Debug("get-state ok")
	return strings.TrimSpace(string(out)), nil
}

// Shell executes a shell command through adb.
func (a *ADB) Shell(cmd ...string) (string, error) {
	fullCmd := append([]string{"shell"}, cmd...)
	out, err := a.Exec(fullCmd...)
	if err != nil {
		if out == nil {
			return "", err
		}
		// Still return a value because some commands returns 1 but still works.
		return strings.TrimSpace(string(out)), err
	}

	return strings.TrimSpace(string(out)), nil
}

// RootShell executes a command through an already-functional su binary. It
// does not attempt to enable or install root access.
func (a *ADB) RootShell(command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("root shell command cannot be empty")
	}
	return a.Shell("su", "-c", command)
}

// HasRoot reports whether su can execute commands as UID 0.
func (a *ADB) HasRoot() bool {
	out, err := a.RootShell("id -u")
	return err == nil && strings.TrimSpace(out) == "0"
}

// FileExistsAsRoot checks a fixed device path without exposing it to shell
// expansion.
func (a *ADB) FileExistsAsRoot(devicePath string) (bool, error) {
	if devicePath == "" {
		return false, fmt.Errorf("device path cannot be empty")
	}
	out, err := a.RootShell("if [ -f " + QuoteRemoteShellArg(devicePath) + " ]; then printf 1; else printf 0; fi")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}

// FileSizeAsRoot returns the size of a root-readable file without copying it.
func (a *ADB) FileSizeAsRoot(devicePath string) (int64, error) {
	if devicePath == "" {
		return 0, fmt.Errorf("device path cannot be empty")
	}
	out, err := a.RootShell("wc -c < " + shellQuote(devicePath))
	if err != nil {
		return 0, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil || size < 0 {
		return 0, fmt.Errorf("invalid file size %q", out)
	}
	return size, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// Pull downloads a file from the device to a local path.
func (a *ADB) Pull(remotePath, localPath string) (string, error) {
	out, err := a.Exec("pull", remotePath, localPath)
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}

// Push a file on the phone
func (a *ADB) Push(localPath, remotePath string) (string, error) {
	out, err := a.Exec("push", localPath, remotePath)
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}

// Backup generates a backup of the specified app or of all, writing the
// archive directly to acquisition dir.
func (a *ADB) Backup(outPath, arg string) error {
	args := []string{"backup", "-nocompress", "-f", outPath, arg}
	if a.Serial != "" {
		args = append([]string{"-s", a.Serial}, args...)
	}
	cmd := a.command(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// Write bugreport directly to acquisition dir.
func (a *ADB) Bugreport(outPath string) error {
	args := []string{"bugreport", outPath}
	if a.Serial != "" {
		args = append([]string{"-s", a.Serial}, args...)
	}
	cmd := a.command(args...)
	err := cmd.Run()
	return err
}

// IL prompts the user to download Intrusion Logs
func (a *ADB) IL() error {
	// adb shell am start -n com.google.android.gms/.intrusiondetection.ui.retrieval.IntrusionDetectionRetrievalActivity
	cmd, err := a.Shell(
		"am",
		"start",
		"-n",
		"com.google.android.gms/.intrusiondetection.ui.retrieval.IntrusionDetectionRetrievalActivity",
	)
	if err != nil {
		return fmt.Errorf("failed to start IL activity: %v: %s", err, cmd)
	}
	return nil
}

// check if file exists
func (a *ADB) FileExists(path string) (bool, error) {
	out, err := a.Shell("[", "-f", QuoteRemoteShellArg(path), "] || echo 1")
	if err != nil {
		return false, err
	}
	if out == "1" {
		return false, nil
	}
	return true, nil
}

// List files in a folder using ls, returns array of strings.
func (a *ADB) ListFiles(remotePath string, recursive bool) ([]string, error) {
	var remoteFiles []string

	// Quote remotePath so shell metacharacters remain part of the path.
	qPath := QuoteRemoteShellArg(remotePath)

	if recursive {
		out, err := a.Shell("find", qPath, "-type", "f", "2>", "/dev/null")
		if out != "" {
			tmpFiles := strings.Split(out, "\n")
			for _, file := range tmpFiles {
				// Remove errors
				if !strings.HasPrefix(file, "find:") {
					remoteFiles = append(remoteFiles, file)
				}
			}
		}
		if err != nil {
			return remoteFiles, err
		}
	} else {
		out, err := a.Shell("ls", qPath)
		if err != nil {
			return remoteFiles, err
		}
		if strings.HasPrefix(out, "ls:") {
			// Error
			return remoteFiles, errors.New(out)
		}
		remoteFiles = strings.Split(out, "\n")
	}

	return remoteFiles, nil
}

func (a *ADB) KillServer() (string, error) {
	log.Debug("Killing adb server")
	out, err := exec.Command(a.ExePath, "kill-server").Output()
	if err != nil {
		log.Debug("kill-server failed")
		return "", err
	}

	log.Debug("kill-server ok")
	return strings.TrimSpace(string(out)), nil
}
