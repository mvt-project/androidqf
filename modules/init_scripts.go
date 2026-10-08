// Copyright (c) 2026 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

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

const initScriptsRoot = "/system/etc/init.d"

type InitScripts struct{}

func NewInitScripts() *InitScripts { return &InitScripts{} }

func (i *InitScripts) Name() string { return "init_scripts" }

func (i *InitScripts) Run(acq *acquisition.Acquisition, opts *Options) error {
	if adb.Client == nil || !adb.Client.HasRoot() {
		log.Info("Skipping init scripts: existing root access is unavailable.")
		return nil
	}

	log.Info("Collecting init.d scripts...")
	// Missing init.d directories are normal. Do not follow symlinks or execute
	// scripts; enumerate regular files, including hidden files and subdirectories.
	out, err := adb.Client.RootShell("if [ -d " + initScriptsRoot + " ]; then find " + initScriptsRoot + "/ -type f -print0; fi")
	var collectionErr error
	if err != nil {
		collectionErr = fmt.Errorf("failed to list init scripts: %w", err)
	}
	for _, devicePath := range strings.Split(out, "\x00") {
		if devicePath == "" {
			continue
		}
		if err := opts.ContextOrBackground().Err(); err != nil {
			return fmt.Errorf("%w: %v", ErrAcquisitionInterrupted, err)
		}
		rel, err := relativeDeviceChild(initScriptsRoot, devicePath)
		if err != nil {
			collectionErr = errors.Join(collectionErr, err)
			continue
		}
		if err := acq.PullRootToZipStaged(devicePath, path.Join("init_scripts", rel)); err != nil {
			log.Warningf("Unable to collect init script %s: %v", devicePath, err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", devicePath, err))
		}
	}
	return partialCollectionError(collectionErr)
}
