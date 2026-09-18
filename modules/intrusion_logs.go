// androidqf - Android Quick Forensics
// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

const (
	acquireIL = "Yes"
	skipIL    = "No"
)

type IL struct {
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

func ParseIntrusionLogsOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes":
		return acquireIL, nil
	case "no":
		return skipIL, nil
	}
	return "", fmt.Errorf("invalid -intrusion-logs value %q (valid values: yes, no)", value)
}

func (m *IL) Run(acq *acquisition.Acquisition, opts *Options) error {
	var collectionErr error

	// Check whether the device supports AAPM.
	compatible, err := m.isAAPMCompatibleDevice()
	if err != nil {
		log.Debugf("Failed to check AAPM compatibility: %v", err)
		return partialCollectionError(fmt.Errorf("failed to check AAPM compatibility: %w", err))
	}

	// TODO: Investigate whether IL data could exist on a non-compatible device
	// (for example, restored or migrated from another device on the same Google account).
	// If so, skipping here might miss existing data.
	if !compatible {
		log.Info("Device is not AAPM-compatible, skipping Intrusion Logging acquisition.")
		return nil
	}

	// Ask user first
	ILOption, err := resolveOption(opts, opts.IntrusionLogs, "-intrusion-logs (yes, no)", func() (string, error) {
		log.Info("Would you like to download Intrusion Logs from the device?")
		promptIL := promptui.Select{
			Label: "Intrusion Logs",
			Items: []string{acquireIL, skipIL},
		}
		_, selection, err := promptIL.Run()
		return selection, err
	})
	if err != nil {
		return fmt.Errorf("failed to make selection for IL option: %v", err)
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
		collectionErr = errors.Join(collectionErr, fmt.Errorf("failed to check AAPM enabled state: %w", err))
		aapmEnabled = false
	}

	if aapmEnabled {
		// Snapshot of Intrusion Logs folder before triggering new log download
		before, err := m.listDirSet(m.DirOnDevice)

		if err != nil {
			log.Errorf("IL: failed to completely list %s before requesting new logs: %v", m.DirOnDevice, err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("failed to list %s before requesting new logs: %w", m.DirOnDevice, err))
		}

		// Start the Activity to prompt the user to download a new Intrusion Log
		if err := adb.Client.IL(); err != nil {
			log.Errorf("Failed to launch intrusion detection activity: %v\n", err)
			collectionErr = errors.Join(collectionErr, err)
			// Still allow pulling existing files if user wants; continue anyway.
		}

		log.Info("Launched the Intrusion Logging settings page.")
		log.Info("On the device: scroll down, tap 'Access Logs', then press 'Download and Decrypt' for each listed device.\n")

		log.Info("Waiting for intrusion logs to be written to device. (Ctrl+C to skip waiting and continue acquisition)...")
		// Pulls every 2 seconds. Stops on Ctrl+C or after 15 minutes.
		watchErr := m.waitForNewFiles(opts.ContextOrBackground(), opts.Signals, m.DirOnDevice, before, 2*time.Second, 15*time.Minute)
		if errors.Is(watchErr, ErrAcquisitionInterrupted) {
			return watchErr
		}
		if watchErr != nil {
			log.Info("Stopped waiting, continuing with acquisition...")
			collectionErr = errors.Join(collectionErr, watchErr)
		}
	} else {
		log.Debug("AAPM is disabled, skipping activity launch and new file watcher (pulling existing files only).")
	}

	// Pull all files (old + new)
	files, err := adb.Client.ListFiles(m.DirOnDevice, true)
	if err != nil {
		collectionErr = errors.Join(collectionErr, fmt.Errorf("failed to list files for pull in %s: %w", m.DirOnDevice, err))
	}
	if len(files) == 0 {
		log.Info("No files found in " + m.DirOnDevice)
		return partialCollectionError(collectionErr)
	}

	if err := m.pullAll(acq, files); err != nil {
		log.Errorf("IL: failed pulling IL files: %v", err)
		collectionErr = errors.Join(collectionErr, err)
	}
	log.Infof("Downloaded %d Instrusion Logging files from the phone.", len(files))
	log.Info("Intrusion Logging acquisition is completed; continuing with acquisition ...")
	return partialCollectionError(collectionErr)
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
	log.Debugf("IL: Polling found %d intrusion logging files on device at '%s'", len(files), dir)
	set := make(map[string]struct{}, len(files))
	for _, f := range files {
		set[f] = struct{}{}
	}
	return set, err
}

// Watch for new files until Ctrl+C or timeout.
func (m *IL) waitForNewFiles(
	ctx context.Context,
	signals <-chan os.Signal,
	dir string,
	before map[string]struct{},
	pollEvery time.Duration,
	maxWait time.Duration,
) error {
	ticker := time.NewTicker(pollEvery)
	timeout := time.NewTimer(maxWait)

	defer ticker.Stop()
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ErrAcquisitionInterrupted, ctx.Err())

		case received := <-signals:
			if received == os.Interrupt {
				log.Info("Ctrl+C detected. Continuing acquisition...")
				return nil
			}
			return fmt.Errorf("%w: received %s", ErrAcquisitionInterrupted, received)

		case <-timeout.C:
			log.Info("Finished waiting for intrusion logs (15 minute timeout reached).")
			return nil

		case <-ticker.C:
			now, err := m.listDirSet(dir)
			if err != nil {
				log.Debugf("IL: poll list failed: %v", err)
				continue
			}

			for f := range now {
				if _, existed := before[f]; !existed {
					log.Infof(
						"Detected new file: %s.\nIf you finished downloading logs, press Ctrl+C to continue acquisition.",
						f,
					)
					before[f] = struct{}{}
				}
			}
		}
	}
}

func (m *IL) pullAll(acq *acquisition.Acquisition, deviceFiles []string) error {
	var collectionErr error
	for _, file := range deviceFiles {
		if file == m.DirOnDevice {
			continue
		}

		rel, err := relativeDeviceChild(m.DirOnDevice, file)
		if err != nil {
			log.Errorf("Skipping IL file with unsafe path %s: %v\n", file, err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", file, err))
			continue
		}

		zipPath := path.Join("intrusion_logs", rel)

		if err := acq.PullToZipStaged(file, zipPath); err != nil {
			log.Errorf("Failed to stage IL file %s for archive: %v\n", file, err)
			collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", file, err))
			continue
		}

		log.Debugf("Staged IL file %s and added it to archive as %s", file, zipPath)
	}

	return collectionErr
}
