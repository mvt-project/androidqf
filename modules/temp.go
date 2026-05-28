// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

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

	streaming := acq.StreamingMode && acq.EncryptedWriter != nil
	var localRoot *os.Root
	var puller *acquisition.StreamingPuller
	if !streaming {
		var err error
		localRoot, err = os.OpenRoot(t.TempPath)
		if err != nil {
			return fmt.Errorf("failed to open tmp output root: %v", err)
		}
		defer localRoot.Close()
		puller = acquisition.NewStreamingPuller(adb.Client.ExePath, adb.Client.Serial, 100)
	}

	// TODO: Also check default tmp folders
	tmpFiles, err := adb.Client.ListFiles(acq.TmpDir, true)
	if err != nil {
		return fmt.Errorf("failed to list files in tmp: %v", err)
	}

	for _, file := range tmpFiles {
		if file == acq.TmpDir {
			continue
		}

		rel, err := relativeDeviceChild(acq.TmpDir, file)
		if err != nil {
			log.Errorf("Skipping temp file with unsafe path %s: %v\n", file, err)
			continue
		}

		if streaming {
			// Streaming mode: stream directly from ADB to encrypted zip without temp files
			zipPath := path.Join("tmp", rel)

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
			// Traditional mode: stream into a file opened relative to t.TempPath.
			if err := streamDeviceChildToRoot(localRoot, puller, rel, file); err != nil {
				if !text.ContainsNoCase(err.Error(), "Permission denied") {
					log.Errorf("Failed to pull temp file %s: %v\n", file, err)
				}
				continue
			}
		}
	}
	return nil
}
