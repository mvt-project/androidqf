// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"fmt"
	"github.com/mvt/androidqf/log"
)

func (a *Acquisition) Settings() error {
	log.Info("Collecting device settings...")

	results := map[string]map[string]string{}
	namespaces := []string{"system", "secure", "global"}

	for _, namespace := range namespaces {
		results[namespace] = map[string]string{}
		out, err := a.ADB.Shell(fmt.Sprintf("cmd settings list %s", namespace))
		if err != nil {
			return fmt.Errorf("failed to run `cmd settings %s`: %v",
				namespace, err)
		}

		err = a.saveOutput(fmt.Sprintf("settings_%s.txt", namespace), out)
		if err != nil {
			return fmt.Errorf("failed to save settings file %s ù %v", namespace, err)
		}
	}

	return nil
}
