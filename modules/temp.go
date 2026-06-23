// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"path"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Temp struct{}

func NewTemp() *Temp {
	return &Temp{}
}

func (t *Temp) Name() string {
	return "temp"
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

		rel, err := relativeDeviceChild(acq.TmpDir, file)
		if err != nil {
			log.Errorf("Skipping temp file with unsafe path %s: %v\n", file, err)
			continue
		}

		zipPath := path.Join("tmp", rel)

		writer, err := acq.ZipWriter.CreateFile(zipPath)
		if err != nil {
			log.Errorf("Failed to create zip entry for temp file %s: %v\n", file, err)
			continue
		}

		err = acq.StreamingPuller.PullToWriter(file, writer)
		if err != nil {
			log.Errorf("Failed to stream temp file %s: %v\n", file, err)
			continue
		}

		log.Debugf("Streamed temp file %s directly to archive as %s", file, zipPath)
	}
	return nil
}
