// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"io"
	"os"
	"path/filepath"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Bugreport struct {
	StoragePath string
}

func NewBugreport() *Bugreport {
	return &Bugreport{}
}

func (b *Bugreport) Name() string {
	return "bugreport"
}

func (b *Bugreport) InitStorage(storagePath string) error {
	b.StoragePath = storagePath
	return nil
}

func (b *Bugreport) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info(
		"Generating a bugreport for the device...",
	)

	bugreportPath := filepath.Join(b.StoragePath, "bugreport.zip")

	if acq.UseMemoryFs {
		// Write bugreport directly to memory filesystem
		bugreportFile, err := acq.Fs.Create(bugreportPath)
		if err != nil {
			return err
		}
		defer bugreportFile.Close()

		err = adb.Client.BugreportToWriter(bugreportFile)
		if err != nil {
			log.Debugf("Impossible to generate bugreport: %v", err)
			return err
		}
	} else {
		// Use traditional disk-based approach
		err := adb.Client.Bugreport()
		if err != nil {
			log.Debugf("Impossible to generate bugreport: %v", err)
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			log.Debugf("Impossible to get current directory: %v", err)
			return err
		}

		origBugreportPath := filepath.Join(cwd, "bugreport.zip")

		// Read the bugreport file from disk
		origFile, err := os.Open(origBugreportPath)
		if err != nil {
			return err
		}
		defer origFile.Close()

		// Write to the filesystem
		bugreportFile, err := acq.Fs.Create(bugreportPath)
		if err != nil {
			return err
		}
		defer bugreportFile.Close()

		// Copy the file content
		_, err = io.Copy(bugreportFile, origFile)
		if err != nil {
			return err
		}

		// Remove the original file from disk
		err = os.Remove(origBugreportPath)
		if err != nil {
			log.Debugf("Failed to remove original bugreport file: %v", err)
		}
	}

	log.Debug("Bugreport completed!")

	return nil
}
