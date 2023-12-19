// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/botherder/go-savetime/text"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Logs struct {
	StoragePath string
	LogsPath    string
}

func NewLogs() *Logs {
	return &Logs{}
}

func (l *Logs) Name() string {
	return "logs"
}

func (l *Logs) InitStorage(storagePath string) error {
	l.StoragePath = storagePath
	l.LogsPath = filepath.Join(storagePath, "logs")
	err := os.Mkdir(l.LogsPath, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create logs folder: %v", err)
	}

	return nil
}

func (l *Logs) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting system logs...")

	logFiles := []string{
		"/data/system/uiderrors.txt",
		"/proc/kmsg",
		"/proc/last_kmsg",
		"/sys/fs/pstore/console-ramoops",
	}

	// FIXME: needed to list files versus pulling folders?
	for _, logFolder := range []string{"/data/anr/", "/data/log/", "/sdcard/log/"} {
		files, err := adb.Client.ListFiles(logFolder, true)
		if err != nil {
			log.Debugf("Impossible to get files from %", logFolder)
			continue
		}
		if len(files) == 0 {
			continue
		}

		logFiles = append(logFiles, files...)
		log.Debugf("Files in %s: %s", logFolder, files)
	}

	for _, logFile := range logFiles {
		localPath := filepath.Join(l.LogsPath, logFile)
		localDir, _ := filepath.Split(localPath)
		log.Debugf("From: %s", logFile)
		log.Debugf("To: %s", localPath)

		err := os.MkdirAll(localDir, 0o755)
		if err != nil {
			log.Errorf("Failed to create folders for logs %s: %v\n", localDir, err)
			continue
		}

		out, err := adb.Client.Pull(logFile, localPath)
		if err != nil {
			if !text.ContainsNoCase(out, "Permission denied") {
				log.Errorf("Failed to pull log file %s: %s\n", logFile, strings.TrimSpace(out))
			}
			continue
		}
	}

	return nil
}
