// androidqf - Android Quick Forensics
// Copyright (c) 2021 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	saveRuntime "github.com/botherder/go-savetime/runtime"
)

type Collector struct {
	ExePath      string
	Installed    bool
	Adb          *ADB
	Architecture string
}

type FileInfo struct {
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	Mode         string `json:"mode"`
	UserId       uint32 `json:"user_id"`
	UserName     string `json:"user_name"`
	GroupId      uint32 `json:"group_id"`
	GroupName    string `json:"group_name"`
	ChangeTime   int64  `json:"changed_time"`
	ModifiedTime int64  `json:"modified_time"`
	AccessTime   int64  `json:"access_time"`
	Error        string `json:"error"`
	Context      string `json:"context"`
	SHA1         string `json:"sha1"`
	SHA256       string `json:"sha256"`
	SHA512       string `json:"sha512"`
	MD5          string `json:"md5"`
}

type ProcessInfo struct {
	Pid              uint32   `json:"pid"`
	Uid              uint32   `json:"uid"`
	Ppid             uint32   `json:"ppid"`
	Pgroup           uint32   `json:"pgroup"`
	Psid             uint32   `json:"psid"`
	Filename         string   `json:"filename"`
	Priority         uint32   `json:"priority"`
	State            string   `json:"state"`
	UserTime         uint32   `json:"user_time"`
	KernelTime       uint32   `json:"kernel_time"`
	Path             string   `json:"path"`
	Context          string   `json:"context"`
	PreviousContext  string   `json:"previous_context"`
	CommandLine      []string `json:"command_line"`
	Environment      []string `json:"env"`
	WorkingDirectory string   `json:"cwd"`
}

// Returns a new Collector instance.
func (a *ADB) GetCollector(tmpDir string, arch string) (*Collector, error) {
	c := Collector{ExePath: filepath.Join(tmpDir, "collector"), Adb: a, Architecture: arch}

	err := c.Install()
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// Check if collector is installed.
func (c *Collector) isInstalled() bool {
	out, err := c.Adb.FileExists(c.ExePath)
	if err != nil {
		return false
	}
	return out
}

// Clean the phone.
func (c *Collector) Clean() error {
	_, err := c.Adb.Shell("rm", c.ExePath)
	return err
}

// Install the collector.
func (c *Collector) Install() error {
	if c.isInstalled() {
		_, err := c.Adb.Shell("rm", c.ExePath)
		if err != nil {
			return err
		}
	}
	if !strings.HasPrefix(c.Architecture, "armeabi-v") && !strings.HasPrefix(c.Architecture, "armeabi-v7") && !strings.HasPrefix(c.Architecture, "arm64-v8") {
		return fmt.Errorf("unsupported architecture: %s", c.Architecture)
	}
	collectorPath := filepath.Join(saveRuntime.GetExecutableDirectory(), "collector_arm6")
	if _, err := os.Stat(collectorPath); err != nil {
		// Somehow the file doesn't exist
		return errors.New("couldn't find the collector binary")
	}

	_, err := c.Adb.Push(collectorPath, c.ExePath)
	if err != nil {
		return err
	}
	_, err = c.Adb.Shell("chmod", "+x", c.ExePath)
	if err != nil {
		return err
	}

	return nil
}

// List files on the phone at the given path (no hash).
func (c *Collector) Find(path string) ([]FileInfo, error) {
	var results []FileInfo
	var file FileInfo
	if !c.isInstalled() {
		c.Install()
	}

	out, err := c.Adb.Shell(c.ExePath, "find", path)
	if err != nil {
		return results, err
	}
	for _, line := range strings.Split(out, "\n") {
		err = json.Unmarshal([]byte(line), &file)
		if err == nil {
			results = append(results, file)
		}
	}

	return results, nil
}

// List files with their hash on the phone at the given path.
func (c *Collector) FindHash(path string) ([]FileInfo, error) {
	var results []FileInfo
	var file FileInfo
	if !c.isInstalled() {
		c.Install()
	}

	out, err := c.Adb.Shell(c.ExePath, "find", "-H", path)
	if err != nil {
		return results, err
	}
	for _, line := range strings.Split(out, "\n") {
		err = json.Unmarshal([]byte(line), &file)
		if err == nil {
			results = append(results, file)
		}
	}

	return results, nil
}

func (c *Collector) Processes() ([]ProcessInfo, error) {
	var results []ProcessInfo

	if c.isInstalled() {
		c.Install()
	}

	out, err := c.Adb.Shell(c.ExePath, "ps")
	if err != nil {
		return results, err
	}
	err = json.Unmarshal([]byte(out), &results)
	if err != nil {
		return results, err
	}

	return results, nil
}
