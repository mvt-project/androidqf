// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"path/filepath"

	"github.com/botherder/go-savetime/slice"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Files struct {
	StoragePath string
}

func NewFiles() *Files {
	return &Files{}
}

func (f *Files) Name() string {
	return "files"
}

func (f *Files) InitStorage(storagePath string) error {
	f.StoragePath = storagePath
	return nil
}

func (f *Files) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting list of files... This might take a while...")
	var fileFounds []string
	var fileDetails []adb.FileInfo

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
			out, err = acq.Collector.Find(folder)
		} else if method == "findfull" {
			out, err = adb.Client.FindFullCommand(folder)
		} else {
			out, err = adb.Client.FindLimitedCommand(folder)
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

	return saveCommandOutputJson(filepath.Join(f.StoragePath, "files.json"), &fileDetails)
}
