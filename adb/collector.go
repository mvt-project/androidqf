// androidqf - Android Quick Forensics
// Copyright (c) 2021 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//	 https://license.mvt.re/1.1/

package adb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvt-project/androidqf/log"

	"github.com/mvt-project/androidqf/assets"
)

var createCollectorTemp = os.CreateTemp

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
	Pid              int      `json:"pid"`
	Uid              int      `json:"uid"`
	Ppid             int      `json:"ppid"`
	Pgroup           int      `json:"pgroup"`
	Psid             int      `json:"psid"`
	Filename         string   `json:"filename"`
	Priority         int      `json:"priority"`
	State            string   `json:"state"`
	UserTime         int64    `json:"user_time"`
	KernelTime       int64    `json:"kernel_time"`
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
	_, err := c.Adb.Shell("rm", QuoteRemoteShellArg(c.ExePath))
	return err
}

// Install the collector.
func (c *Collector) Install() error {
	if c.isInstalled() {
		_, err := c.Adb.Shell("rm", QuoteRemoteShellArg(c.ExePath))
		if err != nil {
			return err
		}
	}

	collectorName, err := collectorNameForArchitecture(c.Architecture)
	if err != nil {
		return err
	}

	log.Debugf("Deploying collector binary '%s' for architecture '%s'.", collectorName, c.Architecture)
	collectorBinary, err := assets.ReadCollectorFile(collectorName)
	if err != nil {
		// Somehow the file doesn't exist
		return errors.New("couldn't find the collector binary")
	}

	collectorTemp, err := createCollectorTemp("", "collector_")
	if err != nil {
		return err
	}
	defer os.Remove(collectorTemp.Name())

	// Write collector binary out to temporary path
	if _, err := collectorTemp.Write(collectorBinary); err != nil {
		collectorTemp.Close()
		return err
	}
	if err := collectorTemp.Close(); err != nil {
		return err
	}

	_, err = c.Adb.Push(collectorTemp.Name(), c.ExePath)
	if err != nil {
		return err
	}
	_, err = c.Adb.Shell("chmod", "+x", QuoteRemoteShellArg(c.ExePath))
	if err != nil {
		return err
	}

	return nil
}

func collectorNameForArchitecture(architecture string) (string, error) {
	switch {
	case strings.HasPrefix(architecture, "armeabi-v"):
		return "collector_arm", nil
	case strings.HasPrefix(architecture, "arm64-v8"):
		return "collector_arm64", nil
	case strings.HasPrefix(architecture, "x86_64"):
		return "collector_amd64", nil
	default:
		return "", fmt.Errorf("unsupported architecture for collector: %s", architecture)
	}
}

// List files on the phone at the given path (no hash).
func (c *Collector) Find(path string) ([]FileInfo, error) {
	var results []FileInfo
	if !c.isInstalled() {
		err := c.Install()
		if err != nil {
			log.Debugf("Impossible to install collector: %v", err)
			return results, err
		}
	}

	out, err := c.Adb.Shell(QuoteRemoteShellArg(c.ExePath), "find", QuoteRemoteShellArg(path))
	if err != nil {
		return results, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var file FileInfo
		if err := json.Unmarshal([]byte(line), &file); err != nil {
			return results, fmt.Errorf("failed to parse collector file record: %w", err)
		}
		results = append(results, file)
	}

	return results, nil
}

// List files with their hash on the phone at the given path.
func (c *Collector) FindHash(path string) ([]FileInfo, error) {
	var results []FileInfo
	if !c.isInstalled() {
		err := c.Install()
		if err != nil {
			log.Debugf("Impossible to install collector: %v", err)
			return results, err
		}
	}

	out, err := c.Adb.Shell(QuoteRemoteShellArg(c.ExePath), "find", "-H", QuoteRemoteShellArg(path))
	if err != nil {
		return results, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var file FileInfo
		if err := json.Unmarshal([]byte(line), &file); err != nil {
			return results, fmt.Errorf("failed to parse collector file record: %w", err)
		}
		results = append(results, file)
	}

	return results, nil
}

func (c *Collector) Processes() ([]ProcessInfo, error) {
	var results []ProcessInfo

	if !c.isInstalled() {
		err := c.Install()
		if err != nil {
			log.Debugf("Impossible to install collector: %v", err)
			return results, err
		}
	}

	out, err := c.Adb.Shell(QuoteRemoteShellArg(c.ExePath), "ps")
	if err != nil {
		return results, err
	}
	err = json.Unmarshal([]byte(out), &results)
	if err != nil {
		return results, err
	}

	return results, nil
}
