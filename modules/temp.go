// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/botherder/go-savetime/text"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Temp struct {
	StoragePath string
	TempPath    string
}

func NewTemp() *Temp {
	return &Temp{}
}

func (t *Temp) Name() string {
	return "temp"
}

func (t *Temp) InitStorage(storagePath string) error {
	t.StoragePath = storagePath
	t.TempPath = filepath.Join(storagePath, "tmp")
	return nil
}

func (t *Temp) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting files in tmp folder...")

	// TODO: Also check default tmp folders
	tmpFiles, err := adb.Client.ListFiles(acq.TmpDir, true)
	if err != nil {
		return fmt.Errorf("failed to list files in tmp: %v", err)
	}

	for _, file := range tmpFiles {
		if file == acq.TmpDir {
			continue
		}
		dest_path := filepath.Join(t.TempPath,
			strings.TrimPrefix(file, acq.TmpDir))

		dest_dir := filepath.Dir(dest_path)
		err := acq.Fs.MkdirAll(dest_dir, 0o755)
		if err != nil {
			log.Errorf("Failed to create directory %s: %v\n", dest_dir, err)
			continue
		}

		if acq.UseMemoryFs {
			// Pull directly to memory filesystem
			targetFile, err := acq.Fs.Create(dest_path)
			if err != nil {
				log.Errorf("Failed to create target file %s: %v\n", dest_path, err)
				continue
			}
			defer targetFile.Close()

			err = adb.Client.PullToWriter(file, targetFile)
			if err != nil {
				if !text.ContainsNoCase(err.Error(), "Permission denied") {
					log.Errorf("Failed to pull temp file %s: %v\n", file, err)
				}
				continue
			}
		} else {
			// Direct pull to disk
			out, err := adb.Client.Pull(file, dest_path)
			if err != nil {
				if !text.ContainsNoCase(out, "Permission denied") {
					log.Errorf("Failed to pull temp file %s: %s\n", file, strings.TrimSpace(out))
				}
				continue
			}
		}
	}
	return nil
}
