// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mvt/androidqf/acquisition"
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/log"
)

type Processes struct {
	StoragePath string
}

func NewProcesses() *Processes {
	return &Processes{}
}

func (p *Processes) Name() string {
	return "processes"
}

func (p *Processes) InitStorage(storagePath string) error {
	p.StoragePath = storagePath
	return nil
}

func (p *Processes) Run(acq *acquisition.Acquisition) error {
	log.Info("Collecting list of running processes...")

	if acq.Collector == nil {
		out, err := adb.Client.Shell("ps -A")
		if err != nil {
			return fmt.Errorf("failed to run `adb shell ps -A`: %v", err)
		}

		return saveCommandOutput(filepath.Join(p.StoragePath, "processes.txt"), out)
	} else {
		out, err := acq.Collector.Processes()
		if err != nil {
			return err
		}
		jsonData, err := json.Marshal(&out)
		if err != nil {
			return fmt.Errorf("failed to convert JSON: %v", err)
		}
		return saveCommandOutput(filepath.Join(p.StoragePath, "processes.txt"), string(jsonData))
	}
}
