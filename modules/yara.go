// Copyright (c) 2025 Anthony Desnos
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

type Yara struct {
	StoragePath string
}

func NewYara() *Yara {
	return &Yara{}
}

func (y *Yara) Name() string {
	return "yara"
}

func (y *Yara) InitStorage(storagePath string) error {
	y.StoragePath = storagePath
	return nil
}

func (y *Yara) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting results of Yara...")
	var fileFounds []string
	var fileDetails []adb.FileInfo

	log.Debug("Using collector to check Yara rules")

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
		log.Debugf("Starting to collect files in '%s'.", folder)

		var out []adb.FileInfo
		var err error

		out, err = acq.Collector.Yara(folder)

		if err == nil {
			for _, s := range out {
				if !slice.Contains(fileFounds, s.Path) {
					fileFounds = append(fileFounds, s.Path)
					fileDetails = append(fileDetails, s)
				}
			}
		}
	}

	return saveCommandOutputJson(filepath.Join(y.StoragePath, "yara.json"), &fileDetails)
}
