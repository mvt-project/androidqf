// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"io"
	"os"
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

	backupPath := filepath.Join(b.StoragePath, "backup.ab")

	if acq.UseMemoryFs {
		// Write backup directly to memory filesystem
		backupFile, err := acq.Fs.Create(backupPath)
		if err != nil {
			return err
		}
		defer backupFile.Close()

		err = adb.Client.BackupToWriter(arg, backupFile)
		if err != nil {
			log.Debugf("Impossible to get backup: %v", err)
			return err
		}
	} else {
		// Use traditional disk-based approach
		err = adb.Client.Backup(arg)
		if err != nil {
			log.Debugf("Impossible to get backup: %v", err)
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			log.Debugf("Impossible to get current directory: %v", err)
			return err
		}

		origBackupPath := filepath.Join(cwd, "backup.ab")

		// Read the backup file from disk
		origFile, err := os.Open(origBackupPath)
		if err != nil {
			return err
		}
		defer origFile.Close()

		// Write to the filesystem
		backupFile, err := acq.Fs.Create(backupPath)
		if err != nil {
			return err
		}
		defer backupFile.Close()

		// Copy the file content
		_, err = io.Copy(backupFile, origFile)
		if err != nil {
			return err
		}

		// Remove the original file from disk
		err = os.Remove(origBackupPath)
		if err != nil {
			log.Debugf("Failed to remove original backup file: %v", err)
		}
	}

	log.Info("Backup completed!")

	return nil
}
