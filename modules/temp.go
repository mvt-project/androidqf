// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	err := os.Mkdir(t.TempPath, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create tmp folder: %v", err)
	}

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

		adb.Client.Pull(file, dest_path)
	}
	return nil
}
