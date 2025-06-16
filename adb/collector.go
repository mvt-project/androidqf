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

	collectorName := ""
	switch {
	case strings.HasPrefix(c.Architecture, "armeabi-v"):
		collectorName = "collector_arm"
	case strings.HasPrefix(c.Architecture, "armeabi-v7"):
		collectorName = "collector_arm"
	case strings.HasPrefix(c.Architecture, "arm64-v8"):
		collectorName = "collector_arm64"
	default:
		return fmt.Errorf("unsupported architecture for collector: %s", c.Architecture)
	}

	log.Debugf("Deploying collector binary '%s' for architecture '%s' in '%s'.", collectorName, c.Architecture, c.ExePath)
	collectorBinary, err := assets.Collector.ReadFile(collectorName)
	if err != nil {
		// Somehow the file doesn't exist
		return errors.New("couldn't find the collector binary")
	}

	collectorTemp, _ := os.CreateTemp("", "collector_")
	if err != nil {
		return err
	}
	defer os.Remove(collectorTemp.Name())

	// Write collector binary out to temporary path
	if _, err := collectorTemp.Write(collectorBinary); err != nil {
		collectorTemp.Close()
		return err
	}

	_, err = c.Adb.Push(collectorTemp.Name(), c.ExePath)
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
		err := c.Install()
		if err != nil {
			log.Debugf("Impossible to install collector: %w", err)
			return results, err
		}
	}

	out, err := c.Adb.Shell(c.ExePath, "find", "--path", path)
	if err != nil {
		return results, err
	}

	for _, line := range strings.Split(out, "\n") {
		log.Info(line)
		err = json.Unmarshal([]byte(line), &file)

		log.Info(err)
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
		err := c.Install()
		if err != nil {
			log.Debugf("Impossible to install collector: %w", err)
			return results, err
		}
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
		err := c.Install()
		if err != nil {
			log.Debugf("Impossible to install collector: %w", err)
			return results, err
		}
	}

	out, err := c.Adb.Shell(c.ExePath, "ps")
	if err != nil {
		return results, err
	}

	log.Debug(out)

	err = json.Unmarshal([]byte(out), &results)
	if err != nil {
		return results, err
	}

	return results, nil
}
