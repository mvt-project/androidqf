// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
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

func (l *Logcat) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting logcat...")

	out, err := adb.Client.Shell("logcat", "-d", "-b", "all", "\"*:V\"")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell logcat`: %v", err)
	}

	err = saveStringToAcquisition(acq, "logcat.txt", out)
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

	return saveStringToAcquisition(acq, "logcat_old.txt", out)
}
