// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

const (
	hashFiles  = "Yes"
	skipHashes = "No"
)

type Files struct{}

type fileFinder interface {
	Find(path string) ([]adb.FileInfo, error)
	FindHash(path string) ([]adb.FileInfo, error)
}

func NewFiles() *Files {
	return &Files{}
}

func (f *Files) Name() string {
	return "files"
}

func ParseHashFilesOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes":
		return hashFiles, nil
	case "no":
		return skipHashes, nil
	}
	return "", fmt.Errorf("invalid -hash-files value %q (valid values: yes, no)", value)
}

func findFiles(collector fileFinder, path string, withHashes bool) ([]adb.FileInfo, error) {
	if withHashes {
		return collector.FindHash(path)
	}
	return collector.Find(path)
}

func (f *Files) Run(acq *acquisition.Acquisition, opts *Options) error {
	hashOption, err := resolveOption(opts, opts.HashFiles, "-hash-files (yes, no)", func() (string, error) {
		log.Info("Would you like to hash files on the device? This is resource-intensive and may cause the collector to stop on some devices.")
		promptHash := promptui.Select{
			Label: "Hash files",
			Items: []string{skipHashes, hashFiles},
		}
		_, selection, err := promptHash.Run()
		return selection, err
	})
	if err != nil {
		return fmt.Errorf("failed to make selection for file hashing option: %v", err)
	}

	log.Info("Collecting list of files... This might take a while...")
	fileFound := make(map[string]struct{})
	var fileDetails []adb.FileInfo
	var collectionErr error

	method := "collector"
	if acq.Collector == nil {
		out, _ := adb.Client.Shell("find '/' -maxdepth 1 -printf '%T@ %m %s %u %g %p\n' 2> /dev/null")
		if (out == "") || (len(out) == 0) {
			method = "findsimple"
			log.Debug("Using simple find to collect list of files")
		} else {
			method = "findfull"
			log.Debug("Using find command to collect list of files")
		}
	} else {
		log.Debug("Using collector to collect list of files")
	}
	if hashOption == hashFiles && method != "collector" {
		log.Warning("File hashing requires the collector, which is unavailable. Continuing without file hashes.")
	}

	folders := []string{
		"/sdcard/", "/system/", "/system_ext/", "/vendor/",
		"/cust/", "/product/", "/apex/", "/data/local/tmp/", "/data/media/0/",
		"/data/misc/radio/", "/data/vendor/secradio/", "/data/log/", "/tmp/", "/", "/data/data/",
	}
	// If tmp folder different from standard tmp, add it to the list
	if acq.TmpDir != "/data/local/tmp/" {
		folders = append(folders, acq.TmpDir)
	}
	if acq.SdCard != "/sdcard/" {
		folders = append(folders, acq.SdCard)
	}

	for _, folder := range folders {
		var out []adb.FileInfo
		var err error
		if method == "collector" {
			out, err = findFiles(acq.Collector, folder, hashOption == hashFiles)
		} else if method == "findfull" {
			out, err = adb.Client.FindFullCommand(folder)
		} else {
			out, err = adb.Client.FindLimitedCommand(folder)
		}

		if err != nil {
			log.Warningf("Failed to collect files under %s: %v", folder, err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", folder, err))
		}
		for _, s := range out {
			if _, exists := fileFound[s.Path]; !exists {
				fileFound[s.Path] = struct{}{}
				fileDetails = append(fileDetails, s)
			}
		}
	}

	saveErr := saveDataToAcquisition(acq, "files.json", &fileDetails)
	if saveErr != nil {
		return errors.Join(collectionErr, saveErr)
	}
	return partialCollectionError(collectionErr)
}
