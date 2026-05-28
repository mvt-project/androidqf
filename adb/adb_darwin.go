//go:build !unbundle

// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvt-project/androidqf/assets"
)

func (a *ADB) findExe() error {
	// Extract the bundled binary into a temp directory so we
	// never try to write next to the executable (which may be /usr/bin or
	// another read-only system path).
	tmpDir, err := os.MkdirTemp("", "androidqf-adb-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir for adb: %v", err)
	}

	if err := assets.DeployAssetsToDir(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to deploy bundled adb: %v", err)
	}

	a.ExePath = filepath.Join(tmpDir, "adb")
	if err := validatePlatformToolsVersion(a.ExePath); err != nil {
		os.RemoveAll(tmpDir)
		return err
	}
	a.TmpAssetsDir = tmpDir
	return nil
}
