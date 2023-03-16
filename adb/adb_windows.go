// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mvt/androidqf/assets"
	"github.com/mvt/androidqf/log"
)

func (a *ADB) findExe() error {
	// TODO: only deploy assets when needed
	err := assets.DeployAssets()
	if err != nil {
		return err
	}

	adbPath, err := exec.LookPath("adb.exe")
	if err == nil {
		a.ExePath = adbPath
	} else {
		// Get path of the current directory
		ex, err := os.Executable()
		if err != nil {
			return err
		}
		// Need full path to bypass go 1.19 restrictions about local path
		a.ExePath = filepath.Join(filepath.Dir(ex), "adb.exe")
		_, err = os.Stat(a.ExePath)
		if err != nil {
			log.Debugf("ADB doesn't exist at %s", a.ExePath)
			return errors.New("Impossible to find ADB")
		}
	}
	return nil
}
