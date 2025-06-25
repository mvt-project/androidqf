// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvt-project/androidqf/acquisition"
)

type Module interface {
	Name() string
	InitStorage(storagePath string) error
	Run(acq *acquisition.Acquisition, fast bool) error
}

func List() []Module {
	return []Module{
		NewBackup(),
		NewPackages(),
		NewGetProp(),
		NewDumpsys(),
		NewProcesses(),
		NewServices(),
		NewBugreport(),
		NewFiles(),
		NewSettings(),
		NewSELinux(),
		NewEnvironment(),
		NewRootBinaries(),
		NewLogcat(),
		NewLogs(),
		NewTemp(),
		NewYara(),
	}
}

func saveCommandOutputJson(filePath string, data any) error {
	jsonData, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to convert JSON: %v", err)
	}
	return saveCommandOutput(filePath, string(jsonData))
}

func saveCommandOutput(filePath, output string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create %s file: %v", filePath, err)
	}
	defer file.Close()

	_, err = file.WriteString(output)
	if err != nil {
		return fmt.Errorf("failed to write command output to %s: %v", filePath, err)
	}

	file.Sync()

	return nil
}
