// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/log"
)

const (
	backupOnlySMS    = "Only SMS"
	backupEverything = "Everything"
	backupNothing    = "No backup"
)

type Backup struct{}

func NewBackup() *Backup {
	return &Backup{}
}

func (b *Backup) Name() string {
	return "backup"
}

func ParseBackupOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sms":
		return backupOnlySMS, nil
	case "all":
		return backupEverything, nil
	case "none":
		return backupNothing, nil
	}
	return "", fmt.Errorf("invalid -backup value %q (valid values: sms, all, none)", value)
}

func (b *Backup) Run(acq *acquisition.Acquisition, opts *Options) error {
	backupOption, err := resolveOption(opts, opts.Backup, "-backup (sms, all, none)", func() (string, error) {
		log.Info("Would you like to take a backup of the device?")
		promptBackup := promptui.Select{
			Label: "Backup",
			Items: []string{backupOnlySMS, backupEverything, backupNothing},
		}
		_, selection, err := promptBackup.Run()
		return selection, err
	})
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

	err = acq.StreamBackupToZip(arg, "backup.ab")
	if err != nil {
		return fmt.Errorf("failed to stream backup to archive: %v", err)
	}

	log.Info("Backup completed!")

	return nil
}
