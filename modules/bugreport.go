// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Bugreport struct{}

func NewBugreport() *Bugreport {
	return &Bugreport{}
}

func (b *Bugreport) Name() string {
	return "bugreport"
}

func (b *Bugreport) Run(acq *acquisition.Acquisition, opts *Options) error {
	// Preserve existing reports before generating another report, which can
	// trigger Android's retention cleanup.
	collectionErr := collectExistingBugreports(acq)
	if collectionErr != nil {
		log.Warningf("Failed to collect some existing bugreports: %v", collectionErr)
	}

	log.Info(
		"Generating a bugreport for the device...",
	)

	err := acq.StreamBugreportToZip("bugreport.zip")
	if err != nil {
		return errors.Join(collectionErr, fmt.Errorf("failed to stream bugreport to archive: %w", err))
	}

	log.Debug("Bugreport completed!")

	return partialCollectionError(collectionErr)
}

func collectExistingBugreports(acq *acquisition.Acquisition) error {
	log.Info("Collecting existing files from /bugreports/...")
	// cd follows the /bugreports symlink without following symlinks inside it.
	// NUL delimiters preserve filenames containing spaces or newlines. A missing
	// directory is normal; an inaccessible directory remains a collection error.
	out, err := adb.Client.Shell("if [ ! -e /bugreports ] && [ ! -L /bugreports ]; then exit 0; fi; cd /bugreports/ && find . -type f -print0")
	var collectionErr error
	if err != nil {
		collectionErr = fmt.Errorf("listing /bugreports/: %w", err)
	}
	for _, name := range strings.Split(out, "\x00") {
		if name == "" {
			continue
		}
		// Validate before joining: path.Join would hide traversal components.
		remotePath := "/bugreports/" + strings.TrimPrefix(name, "./")
		rel, err := relativeDeviceChild("/bugreports/", remotePath)
		if err != nil {
			collectionErr = errors.Join(collectionErr, err)
			continue
		}
		if err := acq.PullToZipStaged(remotePath, path.Join("bugreports", rel)); err != nil {
			collectionErr = errors.Join(collectionErr, fmt.Errorf("collecting %s: %w", remotePath, err))
		}
	}
	return collectionErr
}
