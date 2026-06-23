// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"

	"github.com/botherder/go-savetime/text"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Logs struct{}

func NewLogs() *Logs {
	return &Logs{}
}

func (l *Logs) Name() string {
	return "logs"
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
			log.Debugf("Impossible to get files from %s", logFolder)
			continue
		}
		if len(files) == 0 {
			continue
		}

		logFiles = append(logFiles, files...)
		log.Debugf("Files in %s: %s", logFolder, files)
	}

	for _, logFile := range logFiles {
		log.Debugf("From: %s", logFile)

		zipPath := fmt.Sprintf("logs%s", logFile)
		log.Debugf("To archive as: %s", zipPath)

		writer, err := acq.ZipWriter.CreateFile(zipPath)
		if err != nil {
			log.Errorf("Failed to create zip entry for log %s: %v\n", logFile, err)
			continue
		}

		err = acq.StreamingPuller.PullToWriter(logFile, writer)
		if err != nil {
			if !text.ContainsNoCase(err.Error(), "Permission denied") {
				log.Errorf("Failed to stream log file %s: %v\n", logFile, err)
			}
			continue
		}

		log.Debugf("Streamed log file %s directly to archive", logFile)
	}

	return nil
}
