// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"path/filepath"

	"github.com/mvt/androidqf/acquisition"
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/log"
)

type Logcat struct {
	StoragePath string
}

func NewLogcat() *Logcat {
	return &Logcat{}
}

func (l *Logcat) Name() string {
	return "logcat"
}

func (l *Logcat) InitStorage(storagePath string) error {
	l.StoragePath = storagePath
	return nil
}

func (l *Logcat) Run(acq *acquisition.Acquisition) error {
	log.Info("Collecting logcat...")

	out, err := adb.Client.Shell("logcat", "-d", "-b", "all", "\"*:V\"")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell logcat`: %v", err)
	}

	err = saveCommandOutput(filepath.Join(l.StoragePath, "logcat.txt"), out)
	if err != nil {
		return err
	}

	// logcat from before reboot
	out, err = adb.Client.Shell("logcat", "-L", "-b", "all", "\"*:V\"")
	if err != nil {
		// Often fails, totally normal
		log.Debugf("failed to run `adb shell logcat -L`: %v", err)
		return nil
	}

	return saveCommandOutput(filepath.Join(l.StoragePath, "logcat_old.txt"), out)
}
