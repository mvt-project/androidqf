// androidqf - Android Quick Forensics
// Copyright (c) 2021 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/botherder/go-savetime/slice"
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/log"
)

func (a *Acquisition) FindFullCommand(path string) ([]adb.FileInfo, error) {
	var results []adb.FileInfo
	out, err := a.ADB.Shell("find", fmt.Sprintf("'%s'", path), "-type", "f", "-printf", "'%T@ %m %s %u %g %p\n'", "2>", "/dev/null")

	if err == nil {
		return results, err
	}

	for _, line := range strings.Split(out, "\n") {
		var new_file adb.FileInfo
		s := strings.Fields(line)
		if len(s) == 0 {
			continue
		}
		time, err := strconv.ParseFloat(s[0], 64)
		if err == nil {
			new_file.ModifiedTime = int64(time)
		}
		new_file.Mode = s[1]
		size, err := strconv.ParseInt(s[2], 10, 64)
		if err == nil {
			new_file.Size = size
		}
		new_file.UserName = s[3]
		new_file.GroupName = s[4]
		new_file.Path = strings.Join(s[5:], "/")

		results = append(results, new_file)
	}

	return results, nil
}

func (a *Acquisition) FindLimitedCommand(path string) ([]adb.FileInfo, error) {
	var results []adb.FileInfo
	out, err := a.ADB.Shell("find", fmt.Sprintf("'%s'", path), "-type", "f", "2>", "/dev/null")

	if err != nil {
		return results, err
	}

	for _, line := range strings.Split(out, "\n") {
		var new_file adb.FileInfo
		new_file.Path = line
		results = append(results, new_file)
	}

	return results, nil
}

func (a *Acquisition) GetFiles() error {
	log.Info("Extracting list of files... This might take a while...")
	var fileFounds []string
	var fileDetails []adb.FileInfo

	method := "collector"
	// FIXME: log failed collector install
	if a.Collector == nil {
		out, _ := a.ADB.Shell("find '/' -maxdepth 1 -printf '%T@ %m %s %u %g %p\n' 2> /dev/null")
		if out == "" {
			method = "findsimple"
			log.Debug("Using simple find to collect list of files")
		} else {
			if len(out) == 0 {
				method = "findsimple"
				log.Debug("Using simple find to collect list of files")
			} else {
				method = "findfull"
				log.Debug("Using find command to collect list of files")
			}
		}
	} else {
		log.Debug("Using collector to collect list of files")
	}

	folders := [15]string{"/sdcard/", "/system/", "/system_ext/", "/vendor/",
		"/cust/", "/product/", "/apex/", "/data/local/tmp/", "/data/media/0/",
		"/data/misc/radio/", "/data/vendor/secradio/", "/data/log/", "/tmp/", "/", "/data/data/"}

	for _, folder := range folders {
		var out []adb.FileInfo
		var err error
		if method == "collector" {
			out, err = a.Collector.Find(folder)
		} else if method == "findfull" {
			out, err = a.FindFullCommand(folder)
		} else {
			out, err = a.FindLimitedCommand(folder)
		}

		if err == nil {
			for _, s := range out {
				if !slice.Contains(fileFounds, s.Path) {
					fileFounds = append(fileFounds, s.Path)
					fileDetails = append(fileDetails, s)
				}
			}
		}
	}

	file, err := os.Create(filepath.Join(a.StoragePath, "files.json"))

	if err != nil {
		return fmt.Errorf("failed to create files.json file: %v", err)
	}
	defer file.Close()
	jsonData, err := json.Marshal(&fileDetails)
	if err != nil {
		return fmt.Errorf("failed to convert JSON: %v", err)
	}
	file.WriteString(string(jsonData))
	return nil
}

func (a *Acquisition) GetTmpFolder() error {
	log.Info("collecting data from temp folder")
	//FIXME: collect temp pat h from env variable
	storageFolder := filepath.Join(a.StoragePath, "tmp")
	if _, err := os.Stat(storageFolder); os.IsNotExist(err) {
		os.Mkdir(storageFolder, 0700)
	}

	tmpFiles, err := a.ADB.ListFiles("/data/local/tmp/", true)
	if err != nil {
		return fmt.Errorf("failed to list files in tmp: %v", err)
	}

	for _, file := range tmpFiles {
		if file == "/data/local/tmp/" {
			continue
		}
		dest_path := filepath.Join(storageFolder,
			strings.TrimPrefix(file, "/data/local/tmp/"))

		a.ADB.Pull(file, dest_path)
	}
	return nil
}
