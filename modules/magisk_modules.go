// androidqf - Android Quick Forensics
// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

const (
	acquireMagiskModules = "Yes"
	skipMagiskModules    = "No"
	magiskModulesRoot    = "/data/adb/modules"
	maxModulePropSize    = 1024 * 1024
)

var magiskModuleStateFiles = []string{"disable", "remove", "update"}

type MagiskModules struct{}

type magiskModuleManifestEntry struct {
	DevicePath         string   `json:"device_path"`
	DirectoryName      string   `json:"directory_name"`
	ModulePropPath     string   `json:"module_prop_path,omitempty"`
	ModulePropStatus   string   `json:"module_prop_status"`
	StateFiles         []string `json:"state_files"`
	StateFilesComplete bool     `json:"state_files_complete"`
}

type magiskModulesManifest struct {
	SchemaVersion     int                         `json:"schema_version"`
	GeneratedAt       time.Time                   `json:"generated_at"`
	Status            string                      `json:"status"`
	AcquisitionMethod string                      `json:"acquisition_method"`
	Modules           []magiskModuleManifestEntry `json:"modules"`
}

func NewMagiskModules() *MagiskModules { return &MagiskModules{} }

func (m *MagiskModules) Name() string { return "magisk_modules" }

func ParseMagiskModulesOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes":
		return acquireMagiskModules, nil
	case "no":
		return skipMagiskModules, nil
	}
	return "", fmt.Errorf("invalid -magisk-modules value %q (valid values: yes, no)", value)
}

func (m *MagiskModules) Run(acq *acquisition.Acquisition, opts *Options) error {
	selection, err := resolveOption(opts, opts.MagiskModules, "-magisk-modules (yes, no)", func() (string, error) {
		log.Info("Would you like to collect installed Magisk module metadata? This requires existing root access.")
		prompt := promptui.Select{
			Label: "Magisk modules",
			Items: []string{skipMagiskModules, acquireMagiskModules},
		}
		_, selection, err := prompt.Run()
		return selection, err
	})
	if err != nil {
		return fmt.Errorf("failed to make selection for Magisk modules: %w", err)
	}
	if selection == skipMagiskModules {
		log.Info("Skipping Magisk module extraction...")
		return nil
	}

	manifest := magiskModulesManifest{
		SchemaVersion:     1,
		GeneratedAt:       time.Now().UTC(),
		Status:            "no_modules",
		AcquisitionMethod: "adb exec-out su -c cat",
		Modules:           []magiskModuleManifestEntry{},
	}
	if adb.Client == nil || !adb.Client.HasRoot() {
		manifest.Status = "root_unavailable"
		log.Warning("Magisk module collection requires an already-functional su binary; no rooting was attempted.")
		return saveDataToAcquisition(acq, "magisk_modules/manifest.json", &manifest)
	}

	moduleDirectories, err := listMagiskModuleDirectories()
	if err != nil {
		manifest.Status = "collection_failed"
		log.Warningf("Unable to enumerate Magisk modules: %v", err)
		return saveDataToAcquisition(acq, "magisk_modules/manifest.json", &manifest)
	}

	partial := false
	for index, devicePath := range moduleDirectories {
		rel, err := relativeDeviceChild(magiskModulesRoot, devicePath)
		if err != nil || strings.Contains(rel, "/") {
			partial = true
			log.Warningf("Skipping Magisk module directory with unsafe path %q", devicePath)
			continue
		}

		entry := magiskModuleManifestEntry{
			DevicePath:         devicePath,
			DirectoryName:      rel,
			ModulePropStatus:   "missing",
			StateFiles:         []string{},
			StateFilesComplete: true,
		}

		moduleProp := path.Join(devicePath, "module.prop")
		exists, err := adb.Client.FileExistsAsRoot(moduleProp)
		if err != nil {
			entry.ModulePropStatus = "check_failed"
			partial = true
			log.Warningf("Unable to check Magisk module property file %s: %v", moduleProp, err)
		} else if exists {
			size, err := adb.Client.FileSizeAsRoot(moduleProp)
			if err != nil {
				entry.ModulePropStatus = "size_check_failed"
				partial = true
				log.Warningf("Unable to determine the size of Magisk module property file %s: %v", moduleProp, err)
			} else if size > maxModulePropSize {
				entry.ModulePropStatus = "too_large"
				partial = true
				log.Warningf("Skipping oversized Magisk module property file %s (%d bytes)", moduleProp, size)
			} else {
				archivePath := fmt.Sprintf("magisk_modules/%04d/module.prop", index)
				if err := acq.PullRootToZipStaged(moduleProp, archivePath); err != nil {
					entry.ModulePropStatus = "collection_failed"
					partial = true
					log.Warningf("Unable to collect Magisk module property file %s: %v", moduleProp, err)
				} else {
					entry.ModulePropPath = archivePath
					entry.ModulePropStatus = "collected"
				}
			}
		} else {
			partial = true
		}

		for _, stateFile := range magiskModuleStateFiles {
			statePath := path.Join(devicePath, stateFile)
			exists, err := adb.Client.FileExistsAsRoot(statePath)
			if err != nil {
				entry.StateFilesComplete = false
				partial = true
				log.Warningf("Unable to check Magisk module state file %s: %v", statePath, err)
				continue
			}
			if exists {
				entry.StateFiles = append(entry.StateFiles, stateFile)
			}
		}

		manifest.Modules = append(manifest.Modules, entry)
	}

	switch {
	case len(manifest.Modules) == 0 && partial:
		manifest.Status = "partial"
	case len(manifest.Modules) == 0:
		manifest.Status = "no_modules"
	case partial:
		manifest.Status = "partial"
	default:
		manifest.Status = "collected"
	}

	log.Infof("Collected metadata for %d Magisk modules.", len(manifest.Modules))
	return saveDataToAcquisition(acq, "magisk_modules/manifest.json", &manifest)
}

func listMagiskModuleDirectories() ([]string, error) {
	command := "for module in " + magiskModulesRoot + "/*; do " +
		"if [ -d \"$module\" ]; then printf '%s\\0' \"$module\"; fi; done"
	out, err := adb.Client.RootShell(command)
	if err != nil {
		return nil, err
	}

	var directories []string
	for _, candidate := range strings.Split(out, "\x00") {
		if candidate == "" {
			continue
		}
		directories = append(directories, candidate)
	}
	sort.Strings(directories)
	return directories, nil
}
