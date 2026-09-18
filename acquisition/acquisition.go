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
	"os"
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
	ADBHostPublicKey string              `json:"adb_host_public_key,omitempty"`
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
	ModuleResults    []ModuleResult      `json:"module_results"`
	logBuffer        *bytes.Buffer       `json:"-"`
}

type ModuleResult struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	Started   time.Time `json:"started"`
	Completed time.Time `json:"completed"`
}

// New returns a new Acquisition instance.
func New(path string) (*Acquisition, error) {
	acq := Acquisition{
		UUID:             uuid.New().String(),
		Started:          time.Now().UTC(),
		AndroidQFVersion: utils.Version,
		StreamingMode:    true,
	}
	if hostKey, err := adb.Client.HostPublicKey(); err != nil {
		log.Warningf("Unable to record ADB host public key: %v", err)
	} else {
		acq.ADBHostPublicKey = hostKey
	}

	// Get system information first to get tmp folder
	err := acq.GetSystemInformation()
	if err != nil {
		acq.cleanupRuntime()
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
		acq.cleanupRuntime()
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
		_ = zipWriter.Close()
		_ = os.Remove(zipWriter.GetOutputPath())
		acq.cleanupRuntime()
		return nil, fmt.Errorf("failed to enable writer logging: %v", err)
	}
	acq.closeLog = closeLog

	return &acq, nil
}

func (a *Acquisition) cleanupRuntime() {
	if a.Collector != nil {
		_ = a.Collector.Clean()
	}
	if adb.Client != nil {
		_, _ = adb.Client.KillServer()
	}
	_ = assets.CleanAssets()
}

func (a *Acquisition) Complete() error {
	var completionErr error

	if a.Completed.IsZero() {
		a.Completed = time.Now().UTC()
	}

	if a.ZipWriter != nil {
		if a.ADBHostPublicKey != "" {
			err := a.ZipWriter.CreateFileFromString("adb_host_key.pub", a.ADBHostPublicKey+"\n")
			if err != nil {
				log.ErrorExc("Failed to store ADB host public key in archive", err)
				completionErr = errors.Join(completionErr, fmt.Errorf("failed to store ADB host public key: %w", err))
			}
		}

		// Store acquisition info in the zip
		info, err := json.MarshalIndent(a, "", " ")
		if err != nil {
			log.Error("Failed to marshal acquisition info for archive")
			completionErr = errors.Join(completionErr, fmt.Errorf("failed to marshal acquisition info: %w", err))
		} else {
			err = a.ZipWriter.CreateFileFromBytes("acquisition.json", info)
			if err != nil {
				log.ErrorExc("Failed to store acquisition info in archive", err)
				completionErr = errors.Join(completionErr, fmt.Errorf("failed to store acquisition info: %w", err))
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
				completionErr = errors.Join(completionErr, fmt.Errorf("failed to add command.log: %w", err))
			}
		}

		err = a.ZipWriter.CreateHashList()
		if err != nil {
			log.ErrorExc("Failed to add hashes.csv to archive", err)
			completionErr = errors.Join(completionErr, fmt.Errorf("failed to add hashes.csv: %w", err))
		}

		err = a.ZipWriter.Close()
		if err != nil {
			log.ErrorExc("Failed to close archive", err)
			completionErr = errors.Join(completionErr, fmt.Errorf("failed to close archive: %w", err))
		}
	} else {
		// Ensure log file is closed before cleanup operations
		if a.closeLog != nil {
			defer a.closeLog()
		}
	}

	a.cleanupRuntime()

	return completionErr
}

// PullToZipStaged validates a complete device pull before creating its ZIP
// entry. Encrypted acquisitions use encrypted temporary storage so plaintext is
// never staged on disk.
func (a *Acquisition) PullToZipStaged(remotePath, zipPath string) error {
	return a.pullToZipStaged(remotePath, zipPath, false)
}

// PullRootToZipStaged validates a complete root-readable device pull before
// creating its ZIP entry.
func (a *Acquisition) PullRootToZipStaged(remotePath, zipPath string) error {
	return a.pullToZipStaged(remotePath, zipPath, true)
}

func (a *Acquisition) pullToZipStaged(remotePath, zipPath string, root bool) error {
	if err := a.validateStreamingMode(); err != nil {
		return err
	}

	pull := a.StreamingPuller.PullToWriter
	if root {
		pull = a.StreamingPuller.PullRootToWriter
	}
	return a.stageStreamToZip(zipPath, func(writer io.Writer) error {
		return pull(remotePath, writer)
	})
}

// stageStreamToZip completes and validates a producer before creating its ZIP
// entry. Encrypted acquisitions use authenticated encrypted temporary storage.
func (a *Acquisition) stageStreamToZip(zipPath string, produce func(io.Writer) error) error {
	if produce == nil {
		return fmt.Errorf("stream producer cannot be nil")
	}

	if a.ZipWriter.IsEncrypted() {
		staged, err := createEncryptedTempFile(produce)
		if err != nil {
			return err
		}
		defer staged.Remove()

		reader, err := staged.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		return a.ZipWriter.CreateFileFromReader(zipPath, reader)
	}

	tempFile, err := os.CreateTemp("", "androidqf-stream-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if err := produce(tempFile); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	return a.ZipWriter.CreateFileFromPath(zipPath, tempPath)
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
			log.Debugf("APK %s exceeded streaming buffer limit; staging it before archiving", remotePath)
			if err := a.PullToZipStaged(remotePath, zipPath); err != nil {
				return fmt.Errorf("failed to add APK %q to zip: %w", remotePath, err)
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

	err := a.stageStreamToZip(zipPath, func(writer io.Writer) error {
		return a.StreamingPuller.BackupToWriter(arg, writer)
	})
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

	err := a.stageStreamToZip(zipPath, a.StreamingPuller.BugreportToWriter)
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
