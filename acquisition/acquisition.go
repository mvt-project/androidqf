// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
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
	"github.com/spf13/afero"
)

// Acquisition is the main object containing all phone information
type Acquisition struct {
	UUID             string         `json:"uuid"`
	AndroidQFVersion string         `json:"androidqf_version"`
	StoragePath      string         `json:"storage_path"`
	Started          time.Time      `json:"started"`
	Completed        time.Time      `json:"completed"`
	Collector        *adb.Collector `json:"collector"`
	TmpDir           string         `json:"tmp_dir"`
	SdCard           string         `json:"sdcard"`
	Cpu              string         `json:"cpu"`
	closeLog         func()         `json:"-"`
	Fs               afero.Fs       `json:"-"`
	UseMemoryFs      bool           `json:"use_memory_fs"`
	KeyFilePresent   bool           `json:"key_file_present"`
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

	// Check if key.txt is present
	keyFilePath := filepath.Join(rt.GetExecutableDirectory(), "key.txt")
	if _, err := os.Stat(keyFilePath); err == nil {
		acq.KeyFilePresent = true
		log.Info("Key file detected, checking available memory for secure processing...")

		// Check if we have enough memory (4GB) and key file is present
		if utils.HasSufficientMemory() {
			acq.UseMemoryFs = true
			acq.Fs = afero.NewMemMapFs()
			log.Info("Using in-memory filesystem for secure data processing")

			// Create the directory structure in memory
			err := acq.Fs.MkdirAll(acq.StoragePath, 0o755)
			if err != nil {
				return nil, fmt.Errorf("failed to create acquisition folder in memory: %v", err)
			}
		} else {
			log.Warning("Insufficient memory for in-memory processing, falling back to disk-based storage")
			acq.UseMemoryFs = false
			acq.Fs = afero.NewOsFs()
		}
	} else {
		acq.KeyFilePresent = false
		acq.UseMemoryFs = false
		acq.Fs = afero.NewOsFs()
		log.Debug("No key file present, using standard disk-based storage")
	}

	// For disk-based filesystem, ensure the directory exists
	if !acq.UseMemoryFs {
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

	// Init logging file - always use OS filesystem for logging to ensure it persists
	logPath := filepath.Join(acq.StoragePath, "command.log")
	var closeLog func()
	if acq.UseMemoryFs {
		// For memory filesystem, we still want logs to persist on disk
		// Create the directory on disk if it doesn't exist
		if _, err := os.Stat(acq.StoragePath); os.IsNotExist(err) {
			err := os.MkdirAll(acq.StoragePath, 0o755)
			if err != nil {
				return nil, fmt.Errorf("failed to create logging directory: %v", err)
			}
		}
	}
	closeLog, err = log.EnableFileLog(log.DEBUG, logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to enable file logging: %v", err)
	}

	// Store cleanup function for later use
	acq.closeLog = closeLog

	return &acq, nil
}

func (a *Acquisition) Complete() {
	a.Completed = time.Now().UTC()

	// Ensure log file is closed before cleanup operations
	if a.closeLog != nil {
		defer a.closeLog()
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
	log.Info("Generating list of files hashes...")

	csvFile, err := a.Fs.Create(filepath.Join(a.StoragePath, "hashes.csv"))
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	err = afero.Walk(a.Fs, a.StoragePath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			return nil
		}

		// For disk-based filesystem, make files read only
		if !a.UseMemoryFs {
			os.Chmod(filePath, 0o400)
		}

		// For memory filesystem, we need to read the file and calculate hash manually
		var sha256 string
		if a.UseMemoryFs {
			content, err := afero.ReadFile(a.Fs, filePath)
			if err != nil {
				return err
			}
			sha256, err = hashes.StringSHA256(string(content))
			if err != nil {
				return err
			}
		} else {
			sha256, err = hashes.FileSHA256(filePath)
			if err != nil {
				return err
			}
		}

		err = csvWriter.Write([]string{filePath, sha256})
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (a *Acquisition) StoreInfo() error {
	log.Info("Saving details about acquisition and device...")

	info, err := json.MarshalIndent(a, "", " ")
	if err != nil {
		return fmt.Errorf("failed to json marshal the acquisition details: %v",
			err)
	}

	infoPath := filepath.Join(a.StoragePath, "acquisition.json")

	err = afero.WriteFile(a.Fs, infoPath, info, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write acquisition details to file: %v",
			err)
	}

	return nil
}
