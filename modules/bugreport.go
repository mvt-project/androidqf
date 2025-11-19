// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
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
	// First, pull existing bugreports from /bugreports/ directory
	err := b.pullExistingBugreports(acq)
	if err != nil {
		log.Warningf("Failed to pull existing bugreports: %v", err)
		// Continue with new bugreport generation even if pulling old ones fails
	}

	log.Info(
		"Generating a bugreport for the device...",
	)

	if acq.StreamingMode && acq.EncryptedWriter != nil {
		// Streaming mode: stream bugreport directly to encrypted zip without temp files
		err := acq.StreamBugreportToZip("bugreport.zip")
		if err != nil {
			return fmt.Errorf("failed to stream bugreport to encrypted archive: %v", err)
		}
	} else {
		// Traditional mode: create bugreport file and move to storage directory
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
	}

	log.Debug("Bugreport completed!")

	return nil
}

// pullExistingBugreports pulls the entire /bugreports/ directory from the device
func (b *Bugreport) pullExistingBugreports(acq *acquisition.Acquisition) error {
	log.Info("Pulling existing bugreports from /bugreports/ directory...")

	// Check if /bugreports directory exists on device
	exists, err := adb.Client.FileExists("/bugreports")
	if err != nil {
		return fmt.Errorf("failed to check if /bugreports directory exists: %v", err)
	}

	if !exists {
		log.Debug("No /bugreports directory found on device")
		return nil
	}

	if acq.StreamingMode && acq.EncryptedWriter != nil {
		// Streaming mode: list files and stream each one to encrypted zip
		return b.streamBugreportsDirectory(acq)
	} else {
		// Traditional mode: pull entire directory using adb pull
		return b.pullBugreportsDirectory()
	}
}

// streamBugreportsDirectory streams individual files from /bugreports/ to encrypted zip
func (b *Bugreport) streamBugreportsDirectory(acq *acquisition.Acquisition) error {
	// List files in /bugreports directory
	files, err := adb.Client.ListFiles("/bugreports", false)
	if err != nil {
		return fmt.Errorf("failed to list files in /bugreports directory: %v", err)
	}

	if len(files) == 0 {
		log.Debug("No existing bugreports found in /bugreports directory")
		return nil
	}

	log.Infof("Found %d items in /bugreports, streaming files...", len(files))

	for _, filename := range files {
		// Skip empty entries and current/parent directory references
		if filename == "" || filename == "." || filename == ".." {
			continue
		}

		remotePath := fmt.Sprintf("/bugreports/%s", filename)
		zipPath := fmt.Sprintf("bugreports/%s", filename)

		log.Debugf("Streaming existing bugreport: %s", filename)

		// Create zip entry writer
		writer, err := acq.EncryptedWriter.CreateFile(zipPath)
		if err != nil {
			log.Warningf("Failed to create zip entry for %s: %v", filename, err)
			continue
		}

		// Stream file directly to zip using StreamingPuller
		err = acq.StreamingPuller.PullToWriter(remotePath, writer)
		if err != nil {
			log.Warningf("Failed to stream existing bugreport %s: %v", filename, err)
			continue
		}

		log.Debugf("Successfully streamed existing bugreport: %s", filename)
	}

	log.Info("Finished streaming existing bugreports")
	return nil
}

// pullBugreportsDirectory pulls the entire /bugreports/ directory using adb pull
func (b *Bugreport) pullBugreportsDirectory() error {
	bugreportsDir := filepath.Join(b.StoragePath, "bugreports")

	// Use adb pull to get the entire directory
	output, err := adb.Client.Pull("/bugreports/", bugreportsDir)
	if err != nil {
		return fmt.Errorf("failed to pull /bugreports directory: %v (output: %s)", err, output)
	}

	log.Info("Successfully pulled existing bugreports directory")
	log.Debugf("ADB pull output: %s", output)
	return nil
}
