//go:build unbundle

// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"fmt"
	"os/exec"
	"runtime"
)

func (a *ADB) findExe() error {
	path, err := exec.LookPath(systemADBName())
	if err != nil {
		return fmt.Errorf("unbundle builds require a package-maintained %s on PATH: %v", systemADBName(), err)
	}

	if err := validatePlatformToolsVersion(path); err != nil {
		return err
	}

	a.ExePath = path
	return nil
}

func systemADBName() string {
	if runtime.GOOS == "windows" {
		return "adb.exe"
	}
	return "adb"
}
