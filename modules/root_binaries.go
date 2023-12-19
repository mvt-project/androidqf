// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"path/filepath"
	"strings"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type RootBinaries struct {
	StoragePath string
}

func NewRootBinaries() *RootBinaries {
	return &RootBinaries{}
}

func (r *RootBinaries) Name() string {
	return "root_binaries"
}

func (r *RootBinaries) InitStorage(storagePath string) error {
	r.StoragePath = storagePath
	return nil
}

func (r *RootBinaries) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Checking for traces of rooting")
	root_binaries := []string{
		"su",
		"busybox",
		"supersu",
		"Superuser.apk",
		"KingoUser.apk",
		"SuperSu.apk",
		"magisk",
		"magiskhide",
		"magiskinit",
		"magiskpolicy",
	}
	found_root_binaries := []string{}
	for _, binary := range root_binaries {
		out, err := adb.Client.Shell("which -a ", binary)
		if err != nil {
			// returns 1 if file not found, ignore
			continue
		}
		if out == "" {
			continue
		}
		if strings.Contains(out, "which: not found") {
			continue
		}
		log.Debugf("Found root binary: %s", out)
		found_root_binaries = append(found_root_binaries, out)
	}

	return saveCommandOutputJson(filepath.Join(r.StoragePath, "root_binaries.json"), &found_root_binaries)
}
