// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
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

	if acq.StreamingMode && acq.EncryptedWriter != nil {
		// Streaming mode: stream backup directly to encrypted zip without temp files
		err = acq.StreamBackupToZip(arg, "backup.ab")
		if err != nil {
			return fmt.Errorf("failed to stream backup to encrypted archive: %v", err)
		}
	} else {
		// Traditional mode: write backup directly into acquisition directory
		backupPath := filepath.Join(b.StoragePath, "backup.ab")
		err = adb.Client.Backup(backupPath, arg)
		if err != nil {
			log.Debugf("Impossible to get backup: %v", err)
			return err
		}
	}

	log.Info("Backup completed!")

	return nil
}
