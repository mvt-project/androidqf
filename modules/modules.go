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
		NewLogcat(),
		NewLogs(),
		NewTemp(),
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

// saveDataToAcquisition saves data to either encrypted stream or file based on acquisition mode
func saveDataToAcquisition(acq *acquisition.Acquisition, filename string, data any) error {
	if acq.StreamingMode && acq.EncryptedWriter != nil {
		return saveDataToStream(acq.EncryptedWriter, filename, data)
	}

	// Fall back to traditional file saving
	filePath := filepath.Join(acq.StoragePath, filename)
	return saveCommandOutputJson(filePath, data)
}

// saveStringToAcquisition saves string content to either encrypted stream or file
func saveStringToAcquisition(acq *acquisition.Acquisition, filename, content string) error {
	if acq.StreamingMode && acq.EncryptedWriter != nil {
		return acq.EncryptedWriter.CreateFileFromString(filename, content)
	}

	// Fall back to traditional file saving
	filePath := filepath.Join(acq.StoragePath, filename)
	return saveCommandOutput(filePath, content)
}

// saveBytesToAcquisition saves byte content to either encrypted stream or file
func saveBytesToAcquisition(acq *acquisition.Acquisition, filename string, content []byte) error {
	if acq.StreamingMode && acq.EncryptedWriter != nil {
		return acq.EncryptedWriter.CreateFileFromBytes(filename, content)
	}

	// Fall back to traditional file saving
	filePath := filepath.Join(acq.StoragePath, filename)
	return os.WriteFile(filePath, content, 0644)
}

// saveDataToStream saves JSON data to encrypted zip stream
func saveDataToStream(writer *acquisition.EncryptedZipWriter, filename string, data any) error {
	jsonData, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to convert JSON: %v", err)
	}
	return writer.CreateFileFromString(filename, string(jsonData))
}
