// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	saveSlice "github.com/botherder/go-savetime/slice"
	"github.com/mvt-project/androidqf/log"
)

type ADB struct {
	ExePath string
	Serial  string
}

var Client *ADB

// New returns a new ADB instance.
func New() (*ADB, error) {
	adb := ADB{}
	err := adb.findExe()
	if err != nil {
		return nil, fmt.Errorf("failed to find a usable adb executable: %v",
			err)
	}
	log.Debugf("ADB found at path: %s", adb.ExePath)

	log.Debug("Killing existing ADB server if running")
	adb.KillServer()

	// Confirm that we can call "adb devices" without errors
	_, err = adb.Devices()
	if err != nil {
		return nil, err
	}
	return &adb, nil
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
		// Problem if multiple devices
		if len(devices) > 1 {
			return "", fmt.Errorf("multiple devices connected, please stop AndroidQF and provide a serial number")
		}
		a.Serial = ""
	}
	return a.Serial, nil
}

// List existing devices
func (a *ADB) Devices() ([]string, error) {
	var devices []string
	out, err := exec.Command(a.ExePath, "devices").Output()
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

// Run a command to the given phone using exec
// Returns string and/or error
func (a *ADB) Exec(args ...string) ([]byte, error) {
	if a.Serial == "" {
		return exec.Command(a.ExePath, args...).Output()
	} else {
		var params []string
		params = append(params, "-s", a.Serial)
		params = append(params, args...)
		return exec.Command(a.ExePath, params...).Output()
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

// Backup generates a backup of the specified app, or of all.
func (a *ADB) Backup(arg string) error {
	cmd := exec.Command(a.ExePath, "backup", "-nocompress", arg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// Bugreport generates a bugreport of the the device
func (a *ADB) Bugreport() error {
	cmd := exec.Command(a.ExePath, "bugreport", "bugreport.zip")
	err := cmd.Run()
	return err
}

// check if file exists
func (a *ADB) FileExists(path string) (bool, error) {
	out, err := a.Shell("[", "-f", path, "] || echo 1")
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
	if recursive {
		out, _ := a.Shell("find", remotePath, "2>", "/dev/null")
		if out != "" {
			tmpFiles := strings.Split(out, "\n")
			for _, file := range tmpFiles {
				// Remove errors
				if !strings.HasPrefix(file, "find:") {
					remoteFiles = append(remoteFiles, file)
				}
			}
		}
	} else {
		out, err := a.Shell("ls", remotePath)
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
