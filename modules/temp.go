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

	// Only create directory in traditional mode
	if storagePath != "" {
		err := os.Mkdir(t.TempPath, 0o755)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to create tmp folder: %v", err)
		}
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

		if acq.StreamingMode && acq.EncryptedWriter != nil {
			// Streaming mode: stream directly from ADB to encrypted zip without temp files
			zipPath := fmt.Sprintf("tmp/%s", strings.TrimPrefix(file, acq.TmpDir))

			// Create zip entry writer
			writer, err := acq.EncryptedWriter.CreateFile(zipPath)
			if err != nil {
				log.Errorf("Failed to create zip entry for temp file %s: %v\n", file, err)
				continue
			}

			// Stream temp file directly to encrypted zip using acquisition's streaming puller
			err = acq.StreamingPuller.PullToWriter(file, writer)
			if err != nil {
				log.Errorf("Failed to stream temp file %s: %v\n", file, err)
				continue
			}

			log.Debugf("Streamed temp file %s directly to encrypted archive as %s", file, zipPath)
		} else {
			// Traditional mode: pull directly to local storage
			dest_path := filepath.Join(t.TempPath,
				strings.TrimPrefix(file, acq.TmpDir))

			adb.Client.Pull(file, dest_path)
		}
	}
	return nil
}
