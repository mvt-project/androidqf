// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"context"
	"os/signal"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

const (
	acquireIL = "Yes"
	skipIL  = "No"
)

type IL struct {
	StoragePath string
	ILPath      string
	DirOnDevice string
}

func NewIL() *IL {
	return &IL{
		DirOnDevice: "/sdcard/Download/Intrusion Logging/",
	}
}

func (m *IL) Name() string {
	return "intrusion_logs"
}

func (m *IL) InitStorage(storagePath string) error {
	m.StoragePath = storagePath
	m.ILPath = filepath.Join(storagePath, "intrusion_logs")

	// Only create directory in traditional mode
	if storagePath != "" {
		err := os.Mkdir(m.ILPath, 0o755)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create Intrusion Logging folder: %v", err)
		}
	}

	return nil
}

func (m *IL) Run(acq *acquisition.Acquisition, fast bool) error {
	// Check whether the device supports AAPM.
	compatible, err := m.isAAPMCompatibleDevice()
	if err != nil {
		// Don't break acquisition if the check fails, just log and skip.
		log.Debugf("Failed to check AAPM compatibility: %v", err)
		return nil
	}

	// TODO: Investigate whether IL data could exist on a non-compatible device
	// (for example, restored or migrated from another device on the same Google account).
	// If so, skipping here might miss existing data.
	if !compatible {
		log.Info("Device is not AAPM-compatible, skipping Intrusion Logging acquisition.")
		return nil
	}

	// Ask user first
	log.Info("Would you like to take the Intrusion Logs of the device?")
	promptIL := promptui.Select{
		Label: "Intrusion Logs",
		Items: []string{acquireIL, skipIL},
	}

	_, ILOption, err := promptIL.Run()
	if err != nil {
		return fmt.Errorf("Failed to make selection for IL option: %v", err)
	}

	// User declined so we continue acquisition normally
	if ILOption == skipIL {
		log.Info("Skipping Intrusion Logging extraction...")
		return nil
	}


	// Check whether AAPM is enabled right now. If disabled, don't start the activity
	// or wait for a new file just pull whatever is already present.
	// We still proceed with acquisition because older IL files may remain on disk
	// and should be collected with user consent.
	aapmEnabled, err := m.isAAPMEnabled()
	if err != nil {
		log.Debugf("Failed to check AAPM enabled state: %v", err)
		aapmEnabled = false
	}

	if aapmEnabled {
		// Snapshot of Intrusion Logs folder before triggering new log download
		before, err := m.listDirSet(m.DirOnDevice)

		if err != nil {
			log.Errorf("IL: failed to list %s: %v", m.DirOnDevice, err)
			return nil
		}

		// Start the Activity to prompt the user to download a new Intrusion Log
		if err := adb.Client.IL(); err != nil {
			log.Errorf("IL: failed to start activity: %v", err)
			// Still allow pulling existing files if user wants; continue anyway.
		}

		log.Info("On the device: open the screen that appears, scroll to the 'Download and Decrypt' button, and tap it.")
		log.Info("Waiting for a new file to appear in " + m.DirOnDevice + " (Ctrl+C to skip waiting and continue acquisition)...")


		// Watch directory (Ctrl+C cancels watch but continues acquisition)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		// Pulls every 2 seconds. Timeout after 15 minutes.
		_, watchErr := m.waitForNewFile(ctx, m.DirOnDevice, before, 2*time.Second, 15*time.Minute)
		if watchErr != nil {
			// If user Ctrl+C, context is canceled and acquisition continues
			log.Info("Stopped waiting, continuing with acquisition...")
		}
	} else {
		log.Debug("AAPM is disabled, skipping activity launch and new file watcher (pulling existing files only).")
	}

	// Pull all files (old + new)
	files, err := adb.Client.ListFiles(m.DirOnDevice, true)
	if err != nil {
		log.Errorf("IL: failed to list files for pull in %s: %v", m.DirOnDevice, err)
		return nil
	}
	if len(files) == 0 {
		log.Info("No files found in " + m.DirOnDevice)
		return nil
	}

	if err := m.pullAll(acq, files); err != nil {
		log.Errorf("IL: failed pulling IL files: %v", err)
		// continue acquisition
		return nil
	}

	log.Debug("Intrusion Logging acquisition is completed; continuing with acquisition ...")
	return nil	
}

func (m *IL) isAAPMCompatibleDevice() (bool, error) {
	// adb shell settings get secure advanced_protection_mode
	out, err := adb.Client.Shell("settings", "get", "secure", "advanced_protection_mode")
	if err != nil {
		return false, err
	}

	val := strings.TrimSpace(out)
	// If the key does not exist, Android prints "null". We infer this is not compatible.
	if strings.EqualFold(val, "null") || val == "" {
		return false, nil
	}

	// If it's compatible, it should be "0" or "1" (treat anything non-null as compatible)
	return true, nil
}

func (m *IL) isAAPMEnabled() (bool, error) {
	// adb shell settings get secure advanced_protection_mode
	out, err := adb.Client.Shell("settings", "get", "secure", "advanced_protection_mode")
	if err != nil {
		return false, err
	}

	val := strings.TrimSpace(out)

	// If the key is missing Android returns "null"
	if strings.EqualFold(val, "null") || val == "" {
		return false, nil
	}

	// AAPM is enabled only when the value is exactly "1"
	return val == "1", nil
}

func (m *IL) listDirSet(dir string) (map[string]struct{}, error) {
	files, err := adb.Client.ListFiles(dir, true)
	if err != nil {
		return nil, err
	}
	log.Debugf("IL: poll found %d files", len(files))
	log.Debugf("IL: poll files: %v", files)
	set := make(map[string]struct{}, len(files))
	for _, f := range files {
		set[f] = struct{}{}
	}
	return set, nil
}

func (m *IL) waitForNewFile(
	ctx context.Context,
	dir string,
	before map[string]struct{},
	pollEvery time.Duration,
	maxWait time.Duration,
) (string, error) {
	deadline := time.NewTimer(maxWait)
	ticker := time.NewTicker(pollEvery)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Ctrl+C => continue acquisition (treat as non-fatal)
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("timeout waiting for new IL file")
		case <-ticker.C:
			now, err := m.listDirSet(m.DirOnDevice)
			if err != nil {
				// non-fatal; keep polling
				log.Debugf("IL: poll list failed: %v", err)
				continue
			}
			for f := range now {
				if _, existed := before[f]; !existed {
					log.Info("Detected new file: " + f)
					return f, nil
				}
			}
		}
	}
}

func (m *IL) pullAll(acq *acquisition.Acquisition, deviceFiles []string) error {
	for _, file := range deviceFiles {
		if file == m.DirOnDevice {
			continue
		}

		rel := strings.TrimPrefix(file, m.DirOnDevice)
		rel = strings.TrimPrefix(rel, "/") // optional safety if DirOnDevice lacks trailing /

		if acq.StreamingMode && acq.EncryptedWriter != nil {
			zipPath := fmt.Sprintf("intrusion_logs/%s", rel)

			writer, err := acq.EncryptedWriter.CreateFile(zipPath)
			if err != nil {
				log.Errorf("Failed to create zip entry for IL file %s: %v\n", file, err)
				continue
			}

			err = acq.StreamingPuller.PullToWriter(file, writer)
			if err != nil {
				log.Errorf("Failed to stream IL file %s: %v\n", file, err)
				continue
			}

			log.Debugf("Streamed IL file %s directly to encrypted archive as %s", file, zipPath)
		} else {
			destPath := filepath.Join(m.ILPath, rel)

			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				log.Errorf("Failed to create folders for IL file %s: %v\n", destPath, err)
				continue
			}

			out, err := adb.Client.Pull(file, destPath)
			if err != nil {
				log.Errorf("Failed to pull IL file %s: %s\n", file, strings.TrimSpace(out))
				continue
			}
		}
	}

	return nil
}
