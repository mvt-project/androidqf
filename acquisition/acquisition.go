// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/assets"
	"github.com/mvt-project/androidqf/log"
	"github.com/mvt-project/androidqf/utils"
)

const streamingPullerMemoryLimitMB = 500

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
	ZipWriter        *StreamingZipWriter `json:"-"`
	StreamingMode    bool                `json:"streaming_mode"`
	StreamingPuller  *StreamingPuller    `json:"-"`
	logBuffer        *bytes.Buffer       `json:"-"`
}

// New returns a new Acquisition instance.
func New(path string) (*Acquisition, error) {
	acq := Acquisition{
		UUID:             uuid.New().String(),
		Started:          time.Now().UTC(),
		AndroidQFVersion: utils.Version,
		StreamingMode:    true,
	}

	// Get system information first to get tmp folder
	err := acq.GetSystemInformation()
	if err != nil {
		return nil, err
	}

	coll, err := adb.Client.GetCollector(acq.TmpDir, acq.Cpu)
	if err != nil {
		// Collector install failed, will use find instead
		log.Debugf("failed to upload collector: %v", err)
	}
	acq.Collector = coll

	zipWriter, err := NewStreamingZipWriter(acq.UUID, path)
	if err != nil {
		return nil, err
	}
	acq.ZipWriter = zipWriter
	acq.StoragePath = zipWriter.GetOutputPath()

	// Initialize streaming puller for direct operations.
	acq.StreamingPuller = NewStreamingPuller(adb.Client.ExePath, adb.Client.Serial, streamingPullerMemoryLimitMB)

	// Create buffer for command.log (will be written to archive at completion).
	acq.logBuffer = new(bytes.Buffer)

	closeLog, err := log.EnableWriterLog(log.DEBUG, acq.logBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to enable writer logging: %v", err)
	}
	acq.closeLog = closeLog

	return &acq, nil
}

func (a *Acquisition) Complete() {
	if a.Completed.IsZero() {
		a.Completed = time.Now().UTC()
	}

	if a.ZipWriter != nil {
		// Store acquisition info in the zip
		info, err := json.MarshalIndent(a, "", " ")
		if err != nil {
			log.Error("Failed to marshal acquisition info for archive")
		} else {
			err = a.ZipWriter.CreateFileFromBytes("acquisition.json", info)
			if err != nil {
				log.ErrorExc("Failed to store acquisition info in archive", err)
			}
		}

		// Close log writer to stop writing to buffer
		// After this, logging will only go to stdout
		if a.closeLog != nil {
			a.closeLog()
		}

		// Write buffered command.log to archive
		if a.logBuffer != nil && a.logBuffer.Len() > 0 {
			err = a.ZipWriter.CreateFileFromBytes("command.log", a.logBuffer.Bytes())
			if err != nil {
				log.ErrorExc("Failed to add command.log to archive", err)
			}
		}

		err = a.ZipWriter.CreateHashList()
		if err != nil {
			log.ErrorExc("Failed to add hashes.csv to archive", err)
		}

		err = a.ZipWriter.Close()
		if err != nil {
			log.ErrorExc("Failed to close archive", err)
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
	if adb.Client != nil {
		adb.Client.KillServer()
	}
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

// StreamAPKToZip streams an APK file directly to the zip with certificate processing
func (a *Acquisition) StreamAPKToZip(remotePath, zipPath string, processFunc func(io.Reader) error) error {
	if err := a.validateStreamingMode(); err != nil {
		return err
	}

	if remotePath == "" {
		return fmt.Errorf("remote path cannot be empty")
	}
	if zipPath == "" {
		return fmt.Errorf("zip path cannot be empty")
	}

	// Pull APK data to memory buffer
	buffer, err := a.StreamingPuller.PullToBuffer(remotePath)
	if err != nil {
		if errors.Is(err, ErrStreamingBufferMemoryLimit) && processFunc == nil {
			log.Debugf("APK %s exceeded streaming buffer limit; streaming directly to archive", remotePath)
			writer, err := a.ZipWriter.CreateFile(zipPath)
			if err != nil {
				return fmt.Errorf("failed to create zip entry for APK %q: %v", remotePath, err)
			}
			if err := a.StreamingPuller.PullToWriter(remotePath, writer); err != nil {
				return fmt.Errorf("failed to stream APK %q to zip: %v", remotePath, err)
			}
			return nil
		}
		return fmt.Errorf("failed to pull APK %q: %v", remotePath, err)
	}

	// Process APK if processor provided (e.g., certificate verification)
	if processFunc != nil {
		err = processFunc(buffer.Reader())
		if err != nil {
			return fmt.Errorf("failed to process APK %q: %v", remotePath, err)
		}
	}

	err = a.ZipWriter.CreateFileFromReader(zipPath, buffer.Reader())
	if err != nil {
		return fmt.Errorf("failed to add APK %q to zip: %v", remotePath, err)
	}

	return nil
}

// StreamBackupToZip streams a backup directly to the zip
func (a *Acquisition) StreamBackupToZip(arg, zipPath string) error {
	if err := a.validateStreamingMode(); err != nil {
		return err
	}

	if arg == "" {
		return fmt.Errorf("backup argument cannot be empty")
	}
	if zipPath == "" {
		return fmt.Errorf("zip path cannot be empty")
	}

	// Create zip entry writer
	writer, err := a.ZipWriter.CreateFile(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip entry for backup: %v", err)
	}

	// Stream backup directly to zip
	err = a.StreamingPuller.BackupToWriter(arg, writer)
	if err != nil {
		return fmt.Errorf("failed to stream backup %q to zip: %v", arg, err)
	}

	return nil
}

// StreamBugreportToZip streams a bugreport directly to the zip
func (a *Acquisition) StreamBugreportToZip(zipPath string) error {
	if err := a.validateStreamingMode(); err != nil {
		return err
	}

	if zipPath == "" {
		return fmt.Errorf("zip path cannot be empty")
	}

	// Create zip entry writer
	writer, err := a.ZipWriter.CreateFile(zipPath)
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

// validateStreamingMode checks if streaming mode is properly initialized
func (a *Acquisition) validateStreamingMode() error {
	if !a.StreamingMode {
		return fmt.Errorf("streaming mode not enabled")
	}
	if a.ZipWriter == nil {
		return fmt.Errorf("zip writer not initialized")
	}
	if a.StreamingPuller == nil {
		return fmt.Errorf("streaming puller not initialized")
	}
	return nil
}
