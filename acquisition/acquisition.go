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
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/assets"
	"github.com/mvt/androidqf/log"
)

// Acquisition is the main object containing all phone information
type Acquisition struct {
	UUID        string         `json:"uuid"`
	StoragePath string         `json:"storage_path"`
	Started     time.Time      `json:"started"`
	Completed   time.Time      `json:"completed"`
	Collector   *adb.Collector `json:"collector"`
	TmpDir      string         `json:"tmp_dir"`
	Cpu         string         `json:"cpu"`
}

// New returns a new Acquisition instance.
func New() (*Acquisition, error) {
	acq := Acquisition{
		UUID:    uuid.New().String(),
		Started: time.Now().UTC(),
	}

	acq.StoragePath = filepath.Join(rt.GetExecutableDirectory(), acq.UUID)
	err := os.Mkdir(acq.StoragePath, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create acquisition folder: %v", err)
	}

	// Get system information first to get tmp folder
	err = acq.GetSystemInformation()
	if err != nil {
		return nil, err
	}

	coll, err := adb.Client.GetCollector(acq.TmpDir, acq.Cpu)
	if err != nil {
		log.Debugf("failed to upload collector: %v", err)
		return nil, fmt.Errorf("failed to upload collector: %v", err)
	}
	acq.Collector = coll

	// Init logging file
	logPath := filepath.Join(acq.StoragePath, "command.log")
	log.EnableFileLog(log.DEBUG, logPath)

	return &acq, nil
}

func (a *Acquisition) Complete() {
	a.Completed = time.Now().UTC()

	if a.Collector != nil {
		a.Collector.Clean()
	}

	assets.CleanAssets()
}

/*func (a *Acquisition) initADB() error {
	var err error
	a.ADB, err = adb.New()
	if err != nil {
		log.Debugf("failed to initialize adb: %v", err)
		return fmt.Errorf("failed to initialize adb: %v", err)
	}

	_, err = a.ADB.GetState()
	if err != nil {
		log.Debugf("failed to get adb state: %v", err)
		return fmt.Errorf("failed to get adb state (are you sure a device is connected?): %v",
			err)
	}

	return nil
}*/

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
	tmpFolder := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TMPDIR=") {
			tmpFolder = strings.TrimPrefix(line, "TMPDIR=")
		}
	}
	if tmpFolder == "" {
		tmpFolder = "/data/local/tmp"
	}
	a.TmpDir = tmpFolder
	log.Debugf("Found temp folder/ %s", tmpFolder)
	return nil
}

func (a *Acquisition) HashFiles() error {
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
