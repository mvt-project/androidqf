// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvt/androidqf/acquisition"
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/log"
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

func (t *Temp) Run(acq *acquisition.Acquisition) error {
	log.Info("Collecting files in tmp folder...")

	// FIXME: collect temp path from env variable
	tmpFiles, err := adb.Client.ListFiles("/data/local/tmp/", true)
	if err != nil {
		return fmt.Errorf("failed to list files in tmp: %v", err)
	}

	for _, file := range tmpFiles {
		if file == "/data/local/tmp/" {
			continue
		}
		dest_path := filepath.Join(t.TempPath,
			strings.TrimPrefix(file, "/data/local/tmp/"))

		adb.Client.Pull(file, dest_path)
	}
	return nil
}
