// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		NewMounts(),
		NewLogcat(),
		NewLogs(),
		NewTemp(),
	}
}

// saveDataToAcquisition saves data to either encrypted stream or file based on acquisition mode
func saveDataToAcquisition(acq *acquisition.Acquisition, filename string, data any) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	if acq.StreamingMode && acq.EncryptedWriter != nil {
		return saveDataToStream(acq.EncryptedWriter, filename, data)
	}

	// Fall back to traditional file saving
	filePath := filepath.Join(acq.StoragePath, filename)
	return saveDataToFile(filePath, data)
}

// saveStringToAcquisition saves string content to either encrypted stream or file
func saveStringToAcquisition(acq *acquisition.Acquisition, filename, content string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	if acq.StreamingMode && acq.EncryptedWriter != nil {
		return acq.EncryptedWriter.CreateFileFromString(filename, content)
	}

	// Fall back to traditional file saving
	filePath := filepath.Join(acq.StoragePath, filename)
	return saveStringToFile(filePath, content)
}

// saveDataToStream saves JSON data to encrypted zip stream
func saveDataToStream(writer *acquisition.EncryptedZipWriter, filename string, data any) error {
	if writer == nil {
		return fmt.Errorf("encrypted writer cannot be nil")
	}

	jsonData, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to convert data to JSON: %v", err)
	}
	return writer.CreateFileFromString(filename, string(jsonData))
}

// saveDataToFile saves JSON data to a file (traditional mode)
func saveDataToFile(filePath string, data any) error {
	jsonData, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to convert data to JSON: %v", err)
	}
	return saveStringToFile(filePath, string(jsonData))
}

// saveStringToFile saves string content to a file (traditional mode)
func saveStringToFile(filePath, content string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %v", filePath, err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write content to file %q: %v", filePath, err)
	}

	err = file.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync file %q: %v", filePath, err)
	}

	return nil
}
