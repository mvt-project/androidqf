// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/botherder/go-savetime/text"
	"github.com/mvt/androidqf/log"
)

func (a *Acquisition) Logs() error {
	log.Info("Collecting system logs...")

	logFiles := []string{
		"/data/system/uiderrors.txt",
		"/proc/kmsg",
		"/proc/last_kmsg",
		"/sys/fs/pstore/console-ramoops",
		"/data/anr/",
		"/data/log/",
		"/sdcard/log",
	}

	for _, logFile := range logFiles {
		localPath := filepath.Join(a.LogsPath, logFile)
		localDir, _ := filepath.Split(localPath)

		err := os.MkdirAll(localDir, 0755)
		if err != nil {
			log.Errorf("Failed to create folders for logs %s: %v", localDir, err)
			continue
		}

		out, err := a.ADB.Pull(logFile, localPath)
		if err != nil {
			if !text.ContainsNoCase(out, "Permission denied") {
				log.Errorf("Failed to pull log file %s: %s", logFile, strings.TrimSpace(out))
			} else {
				log.Debugf("Permission denied to access %s: %s", logFiles, strings.TrimSpace(out))
			}
		}
	}

	return nil
}
