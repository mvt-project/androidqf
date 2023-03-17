// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"encoding/json"
	"fmt"
	"github.com/mvt/androidqf/log"
	"os"
	"path/filepath"
)

func (a *Acquisition) Processes() error {
	log.Info("Collecting list of running processes...")

	if a.Collector == nil {
		out, err := a.ADB.Shell("ps -A")
		if err != nil {
			return fmt.Errorf("failed to run `adb shell ps -A`: %v", err)
		}

		return a.saveOutput("ps.txt", out)
	} else {
		out, err := a.Collector.Processes()
		if err != nil {
			return err
		}

		file, err := os.Create(filepath.Join(a.StoragePath, "processes.json"))

		if err != nil {
			return fmt.Errorf("failed to create processes.json file: %v", err)
		}
		defer file.Close()
		jsonData, err := json.Marshal(&out)
		if err != nil {
			return fmt.Errorf("failed to convert JSON: %v", err)
		}
		file.WriteString(string(jsonData))
		return nil
	}
}
