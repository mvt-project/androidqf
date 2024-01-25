// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
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

	err := adb.Client.Bugreport()
	if err != nil {
		log.Debugf("Impossible to generate bugreport: %w", err)
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Debugf("Impossible to get current directory: %w", err)
		return err
	}

	origBugreportPath := filepath.Join(cwd, "bugreport.zip")
	bugreportPath := filepath.Join(b.StoragePath, "bugreport.zip")

	err = os.Rename(origBugreportPath, bugreportPath)
	if err != nil {
		return err
	}

	log.Debug("Bugreport completed!")

	return nil
}
