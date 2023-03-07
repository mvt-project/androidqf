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
)

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

func (a *Acquisition) FindFullCommand(path string) ([]FileInfo, error) {
	var results []FileInfo
	out, err := a.ADB.Shell("find", fmt.Sprintf("'%s'", path), "-type", "f", "-printf", "'%T@ %m %s %u %g %p\n'", "2>", "/dev/null")

	if err == nil {
		return results, err
	}

	for _, line := range strings.Split(out, "\n") {
		var new_file FileInfo
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

func (a *Acquisition) FindLimitedCommand(path string) ([]FileInfo, error) {
	var results []FileInfo
	out, err := a.ADB.Shell("find", fmt.Sprintf("'%s'", path), "-type", "f", "2>", "/dev/null")

	if err != nil {
		return results, err
	}

	for _, line := range strings.Split(out, "\n") {
		var new_file FileInfo
		new_file.Path = line
		results = append(results, new_file)
	}

	return results, nil
}

func (a *Acquisition) GetFiles() error {
	fmt.Println("Extracting list of files... This might take a while...")
	var fileFounds []string
	var fileDetails []FileInfo
	var method string

	out, _ := a.ADB.Shell("find '/' -maxdepth 1 -printf '%T@ %m %s %u %g %p\n' 2> /dev/null")
	if out == "" {
		method = "findsimple"
	} else {
		if len(out) == 0 {
			method = "findsimple"
		} else {
			method = "findfull"
		}
	}

	folders := [13]string{"/sdcard/", "/system/", "/system_ext/", "/vendor/",
		"/cust/", "/product/", "/apex/", "/data/local/tmp/", "/data/media/0/",
		"/data/misc/radio/", "/data/vendor/secradio/", "/data/log/", "/tmp/"}

	for _, folder := range folders {
		var out []FileInfo
		var err error
		if method == "findfull" {
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

	other_folders := [2]string{"/", "/data/data/"}

	for _, folder := range other_folders {
		var out []FileInfo
		var err error
		if method == "findfull" {
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
