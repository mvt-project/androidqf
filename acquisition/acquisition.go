// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/botherder/go-savetime/hashes"
	rt "github.com/botherder/go-savetime/runtime"
	"github.com/google/uuid"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/assets"
	"github.com/mvt-project/androidqf/log"
	"github.com/mvt-project/androidqf/utils"
)

// Acquisition is the main object containing all phone information
type Acquisition struct {
	UUID             string              `json:"uuid"`
	AndroidQFVersion string              `json:"androidqf_version"`
	StoragePath      string              `json:"storage_path"`
	Started          time.Time           `json:"started"`
	Completed        time.Time           `json:"completed"`
	Collector        *adb.Collector      `json:"collector"`
	TmpDir           string              `json:"tmp_dir"`
	SdCard           string              `json:"sdcard"`
	Cpu              string              `json:"cpu"`
	closeLog         func()              `json:"-"`
	EncryptedWriter  *EncryptedZipWriter `json:"-"`
	StreamingMode    bool                `json:"streaming_mode"`
	StreamingPuller  *StreamingPuller    `json:"-"`
}

// New returns a new Acquisition instance.
func New(path string) (*Acquisition, error) {
	acq := Acquisition{
		UUID:             uuid.New().String(),
		Started:          time.Now().UTC(),
		AndroidQFVersion: utils.Version,
	}

	if path == "" {
		acq.StoragePath = filepath.Join(rt.GetExecutableDirectory(), acq.UUID)
	} else {
		acq.StoragePath = path
	}
	// Check if the path exist
	stat, err := os.Stat(acq.StoragePath)
	if os.IsNotExist(err) {
		err := os.Mkdir(acq.StoragePath, 0o755)
		if err != nil {
			return nil, fmt.Errorf("failed to create acquisition folder: %v", err)
		}
	} else {
		if !stat.IsDir() {
			return nil, fmt.Errorf("path exist and is not a folder")
		}
	}

	// Get system information first to get tmp folder
	err = acq.GetSystemInformation()
	if err != nil {
		return nil, err
	}

	coll, err := adb.Client.GetCollector(acq.TmpDir, acq.Cpu)
	if err != nil {
		// Collector install failed, will use find instead
		log.Debugf("failed to upload collector: %v", err)
	}
	acq.Collector = coll

	// Try to initialize encrypted streaming mode
	encWriter, err := NewEncryptedZipWriter(acq.UUID)
	if err != nil {
		// No key file or encryption setup failed, use normal mode
		log.Debug("Encrypted streaming not available, using normal mode")
		acq.StreamingMode = false

		// Init logging file for normal mode
		logPath := filepath.Join(acq.StoragePath, "command.log")
		closeLog, err := log.EnableFileLog(log.DEBUG, logPath)
		if err != nil {
			return nil, fmt.Errorf("failed to enable file logging: %v", err)
		}
		acq.closeLog = closeLog
	} else {
		// Encrypted streaming mode enabled
		log.Info("Using encrypted streaming mode - data will be written directly to encrypted archive")
		acq.StreamingMode = true
		acq.EncryptedWriter = encWriter
		acq.closeLog = nil // No separate log file in streaming mode

		// Initialize streaming puller for direct operations
		acq.StreamingPuller = NewStreamingPuller(adb.Client.ExePath, 100) // 100MB max memory
	}

	return &acq, nil
}

func (a *Acquisition) Complete() {
	a.Completed = time.Now().UTC()

	// Handle streaming mode completion
	if a.StreamingMode && a.EncryptedWriter != nil {
		// Store acquisition info in the encrypted zip
		info, err := json.MarshalIndent(a, "", " ")
		if err != nil {
			log.Error("Failed to marshal acquisition info for encrypted archive")
		} else {
			err = a.EncryptedWriter.CreateFileFromBytes("acquisition.json", info)
			if err != nil {
				log.ErrorExc("Failed to store acquisition info in encrypted archive", err)
			}
		}

		// Close the encrypted writer
		err = a.EncryptedWriter.Close()
		if err != nil {
			log.ErrorExc("Failed to close encrypted archive", err)
		}

		// Remove the temporary storage directory if it was created and used
		if a.StoragePath != "" {
			if _, err := os.Stat(a.StoragePath); err == nil {
				err = os.RemoveAll(a.StoragePath)
				if err != nil {
					log.ErrorExc("Failed to clean up temporary storage directory", err)
				}
			}
		}
	} else {
		// Ensure log file is closed before cleanup operations
		if a.closeLog != nil {
			defer a.closeLog()
		}
	}

	if a.Collector != nil {
		a.Collector.Clean()
	}

	// Stop ADB server before trying to remove extracted assets
	adb.Client.KillServer()
	assets.CleanAssets()
}

func (a *Acquisition) GetSystemInformation() error {
	// Get architecture information
	out, err := adb.Client.Shell("getprop ro.product.cpu.abi")
	if err != nil {
		return err
	}
	a.Cpu = out
	log.Debugf("CPU architecture: %s", a.Cpu)

	// Get tmp folder
	out, err = adb.Client.Shell("env")
	if err != nil {
		return fmt.Errorf("failed to run `adb shell env`: %v", err)
	}
	a.TmpDir = "/data/local/tmp/"
	a.SdCard = "/sdcard/"
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TMPDIR=") {
			a.TmpDir = strings.TrimPrefix(line, "TMPDIR=")
		}
		if strings.HasPrefix(line, "EXTERNAL_STORAGE=") {
			a.SdCard = strings.TrimPrefix(line, "EXTERNAL_STORAGE=")
		}
	}
	if !strings.HasSuffix(a.TmpDir, "/") {
		a.TmpDir = a.TmpDir + "/"
	}
	if !strings.HasSuffix(a.SdCard, "/") {
		a.SdCard = a.SdCard + "/"
	}

	log.Debugf("Found temp folder at %s", a.TmpDir)
	log.Debugf("Found sdcard at %s", a.SdCard)
	return nil
}

func (a *Acquisition) HashFiles() error {
	// In streaming mode, files are directly encrypted and no local files exist to hash
	if a.StreamingMode {
		log.Debug("Skipping hash generation in streaming mode (data is encrypted)")
		return nil
	}

	log.Info("Generating list of files hashes...")

	csvFile, err := os.Create(filepath.Join(a.StoragePath, "hashes.csv"))
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	_ = filepath.Walk(a.StoragePath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			return nil
		}
		// Makes files read only
		os.Chmod(filePath, 0o400)

		sha256, err := hashes.FileSHA256(filePath)
		if err != nil {
			return err
		}

		err = csvWriter.Write([]string{filePath, sha256})
		if err != nil {
			return err
		}

		return nil
	})

	return nil
}

func (a *Acquisition) StoreInfo() error {
	// In streaming mode, info is stored during Complete()
	if a.StreamingMode {
		return nil
	}

	log.Info("Saving details about acquisition and device...")

	info, err := json.MarshalIndent(a, "", " ")
	if err != nil {
		return fmt.Errorf("failed to json marshal the acquisition details: %v",
			err)
	}

	infoPath := filepath.Join(a.StoragePath, "acquisition.json")

	err = os.WriteFile(infoPath, info, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write acquisition details to file: %v",
			err)
	}

	return nil
}

// StreamAPKToZip streams an APK file directly to encrypted zip with certificate processing
func (a *Acquisition) StreamAPKToZip(remotePath, zipPath string, processFunc func(io.Reader) error) error {
	if !a.StreamingMode || a.EncryptedWriter == nil {
		return fmt.Errorf("streaming mode not available")
	}

	// Pull APK data to memory buffer
	buffer, err := a.StreamingPuller.PullToBuffer(remotePath)
	if err != nil {
		return fmt.Errorf("failed to pull APK %s: %v", remotePath, err)
	}

	// Process APK if processor provided (e.g., certificate verification)
	if processFunc != nil {
		err = processFunc(buffer.Reader())
		if err != nil {
			return fmt.Errorf("failed to process APK %s: %v", remotePath, err)
		}
	}

	// Stream to encrypted zip
	err = a.EncryptedWriter.CreateFileFromReader(zipPath, buffer.Reader())
	if err != nil {
		return fmt.Errorf("failed to add APK %s to encrypted zip: %v", remotePath, err)
	}

	return nil
}

// StreamBackupToZip streams a backup directly to encrypted zip
func (a *Acquisition) StreamBackupToZip(arg, zipPath string) error {
	if !a.StreamingMode || a.EncryptedWriter == nil {
		return fmt.Errorf("streaming mode not available")
	}

	// Create zip entry writer
	writer, err := a.EncryptedWriter.CreateFile(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip entry for backup: %v", err)
	}

	// Stream backup directly to zip
	err = a.StreamingPuller.BackupToWriter(arg, writer)
	if err != nil {
		return fmt.Errorf("failed to stream backup to zip: %v", err)
	}

	return nil
}

// StreamBugreportToZip streams a bugreport directly to encrypted zip
func (a *Acquisition) StreamBugreportToZip(zipPath string) error {
	if !a.StreamingMode || a.EncryptedWriter == nil {
		return fmt.Errorf("streaming mode not available")
	}

	// Create zip entry writer
	writer, err := a.EncryptedWriter.CreateFile(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip entry for bugreport: %v", err)
	}

	// Stream bugreport directly to zip
	err = a.StreamingPuller.BugreportToWriter(writer)
	if err != nil {
		return fmt.Errorf("failed to stream bugreport to zip: %v", err)
	}

	return nil
}
