// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/mvt/androidqf/acquisition"
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/log"
)

const (
	backupOnlySMS    = "Only SMS"
	backupEverything = "Everything"
	backupNothing    = "No backup"
)

type Backup struct {
	StoragePath string
}

func NewBackup() *Backup {
	return &Backup{}
}

func (b *Backup) Name() string {
	return "backup"
}

func (b *Backup) InitStorage(storagePath string) error {
	b.StoragePath = storagePath
	return nil
}

func (b *Backup) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Would you like to take a backup of the device?")
	promptBackup := promptui.Select{
		Label: "Backup",
		Items: []string{backupOnlySMS, backupEverything, backupNothing},
	}
	_, backupOption, err := promptBackup.Run()
	if err != nil {
		return fmt.Errorf("failed to make selection for backup option: %v", err)
	}

	var arg string
	switch backupOption {
	case backupOnlySMS:
		arg = "com.android.providers.telephony"
	case backupEverything:
		arg = "-all"
	case backupNothing:
		return nil
	}

	log.Infof(
		"Generating a backup with argument %s. Please check the device to authorize the backup...\n",
		arg,
	)

	err = adb.Client.Backup(arg)
	if err != nil {
		log.Debugf("Impossible to get backup: %w", err)
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Debugf("Impossible to get current directory: %w", err)
		return err
	}

	origBackupPath := filepath.Join(cwd, "backup.ab")
	backupPath := filepath.Join(b.StoragePath, "backup.ab")

	err = os.Rename(origBackupPath, backupPath)
	if err != nil {
		return err
	}

	log.Info("Backup completed!")

	return nil
}
