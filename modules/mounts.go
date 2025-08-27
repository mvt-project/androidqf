// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"slices"
	"path/filepath"
	"strings"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Mounts struct {
	StoragePath string
}

func NewMounts() *Mounts {
	return &Mounts{}
}

func (m *Mounts) Name() string {
	return "mounts"
}

func (m *Mounts) InitStorage(storagePath string) error {
	m.StoragePath = storagePath
	return nil
}

func (m *Mounts) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting mount information")

	var mountsData []string

	// Run "mount | grep '/system'"
	log.Debug("Running: mount | grep '/system'")
	out1, err1 := adb.Client.Shell("mount | grep '/system'")
	if err1 == nil && out1 != "" {
		lines := strings.Split(strings.TrimSpace(out1), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				mountsData = append(mountsData, strings.TrimSpace(line))
			}
		}
	} else {
		log.Debug("mount | grep '/system' command failed or returned empty result")
	}

	// Run "cat /proc/mounts | grep '/system'"
	log.Debug("Running: cat /proc/mounts | grep '/system'")
	out2, err2 := adb.Client.Shell("cat /proc/mounts | grep '/system'")
	if err2 == nil && out2 != "" {
		lines := strings.Split(strings.TrimSpace(out2), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				trimmedLine := strings.TrimSpace(line)
				// Avoid duplicates
				found := slices.Contains(mountsData, trimmedLine)
				if !found {
					mountsData = append(mountsData, trimmedLine)
				}
			}
		}
	} else {
		log.Debug("cat /proc/mounts | grep '/system' command failed or returned empty result")
	}

	log.Debugf("Found %d mount entries", len(mountsData))

	return saveCommandOutputJson(filepath.Join(m.StoragePath, "mounts.json"), &mountsData)
}
