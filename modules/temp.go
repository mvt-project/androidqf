// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"errors"
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

func (t *Temp) Run(acq *acquisition.Acquisition, opts *Options) error {
	log.Info("Collecting files in tmp folder...")
	var collectionErr error

	// TODO: Also check default tmp folders
	tmpFiles, err := adb.Client.ListFiles(acq.TmpDir, true)
	if err != nil {
		collectionErr = errors.Join(collectionErr, fmt.Errorf("failed to list files in tmp: %w", err))
	}

	for _, file := range tmpFiles {
		if file == acq.TmpDir {
			continue
		}

		rel, err := relativeDeviceChild(acq.TmpDir, file)
		if err != nil {
			log.Errorf("Skipping temp file with unsafe path %s: %v\n", file, err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", file, err))
			continue
		}

		zipPath := path.Join("tmp", rel)

		if err := acq.PullToZipStaged(file, zipPath); err != nil {
			log.Errorf("Failed to stage temp file %s for archive: %v\n", file, err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", file, err))
			continue
		}

		log.Debugf("Staged temp file %s and added it to archive as %s", file, zipPath)
	}
	return partialCollectionError(collectionErr)
}
