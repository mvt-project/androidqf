// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"fmt"
	"github.com/mvt/androidqf/log"
)

func (a *Acquisition) GetProp() error {
	log.Info("Collecting device properties...")

	out, err := a.ADB.Shell("getprop")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell getprop`: %v", err)
	}

	return a.saveOutput("getprop.txt", out)
}
