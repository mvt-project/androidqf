// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mvt-project/androidqf/assets"
	"github.com/mvt-project/androidqf/log"
)

func (a *ADB) findExe() error {
	// Prefer a system-installed adb (covers distro packages where adb is on PATH).
	if path, err := exec.LookPath("adb.exe"); err == nil {
		a.ExePath = path
		return nil
	}

	// Fall back to the bundled binary. Extract it (and the required DLLs) into
	// a temp directory so we never try to write next to the executable (which
	// may be a read-only system path).
	tmpDir, err := os.MkdirTemp("", "androidqf-adb-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir for adb: %v", err)
	}

	if err := assets.DeployAssetsToDir(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to deploy bundled adb: %v", err)
	}

	// Need full path to bypass Go 1.19+ restrictions about relative executable paths.
	exePath := filepath.Join(tmpDir, "adb.exe")
	if _, err := os.Stat(exePath); err != nil {
		os.RemoveAll(tmpDir)
		log.Debugf("ADB doesn't exist at %s", exePath)
		return errors.New("impossible to find ADB")
	}

	a.ExePath = exePath
	a.TmpAssetsDir = tmpDir
	return nil
}
