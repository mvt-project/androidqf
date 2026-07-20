// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"encoding/json"
	"fmt"

	"github.com/mvt-project/androidqf/acquisition"
)

type Module interface {
	Name() string
	Run(acq *acquisition.Acquisition, fast bool) error
}

func List() []Module {
	return []Module{
		NewBackup(),
		NewIL(),
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
		NewMounts(),
		NewLogcat(),
		NewLogs(),
		NewTemp(),
	}
}

// saveDataToAcquisition saves JSON data to the acquisition archive.
func saveDataToAcquisition(acq *acquisition.Acquisition, filename string, data any) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	if acq.ZipWriter == nil {
		return fmt.Errorf("zip writer cannot be nil")
	}
	return saveDataToStream(acq.ZipWriter, filename, data)
}

// saveStringToAcquisition saves string content to the acquisition archive.
func saveStringToAcquisition(acq *acquisition.Acquisition, filename, content string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if acq.ZipWriter == nil {
		return fmt.Errorf("zip writer cannot be nil")
	}
	return acq.ZipWriter.CreateFileFromString(filename, content)
}

// saveDataToStream saves JSON data to a zip stream.
func saveDataToStream(writer *acquisition.StreamingZipWriter, filename string, data any) error {
	if writer == nil {
		return fmt.Errorf("zip writer cannot be nil")
	}

	jsonData, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to convert data to JSON: %v", err)
	}
	return writer.CreateFileFromString(filename, string(jsonData))
}
