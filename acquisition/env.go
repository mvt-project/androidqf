// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"fmt"
)

func (a *Acquisition) GetEnv() error {
	fmt.Println("Collecting environment...")

	out, err := a.ADB.Shell("env")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell env`: %v", err)
	}

	return a.saveOutput("env.txt", out)
}
