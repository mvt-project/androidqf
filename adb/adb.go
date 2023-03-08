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
)

type ADB struct {
	ExePath string
}

// New returns a new ADB instance.
func New() (*ADB, error) {
	adb := ADB{}
	err := adb.findExe()
	if err != nil {
		return nil, fmt.Errorf("failed to find a usable adb executable: %v",
			err)
	}

	return &adb, nil
}

// GetState returns the output of `adb get-state`.
// It is used to check whether a device is connected. If it is not, adb
// will exit with status 1.
func (a *ADB) GetState() (string, error) {
	out, err := exec.Command(a.ExePath, "get-state").Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// Shell executes a shell command through adb.
func (a *ADB) Shell(cmd ...string) (string, error) {
	fullCmd := append([]string{"shell"}, cmd...)
	out, err := exec.Command(a.ExePath, fullCmd...).Output()

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
	out, err := exec.Command(a.ExePath, "pull", remotePath, localPath).Output()
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}

// Push a file on the phone
func (a *ADB) Push(localPath, remotePath string) (string, error) {
	out, err := exec.Command(a.ExePath, "push", localPath, remotePath).Output()
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}

// Backup generates a backup of the specified app, or of all.
func (a *ADB) Backup(arg string) error {
	cmd := exec.Command(a.ExePath, "backup", "-nocompress", arg)
	return cmd.Run()
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
