// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"fmt"
)

func (a *Acquisition) Logcat() error {
	fmt.Println("Collecting logcat...")

	out, err := a.ADB.Shell("logcat", "-d", "-b", "all", "\"*:V\"")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell logcat`: %v", err)
	}
	a.saveOutput("logcat.txt", out)

    // Also get logcat information from before reboot
	out, err = a.ADB.Shell("logcat", "-L", "-b", "all", "\"*:V\"")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell logcat -L`: %v", err)
	}
	return a.saveOutput("logcat_old.txt", out)
}
