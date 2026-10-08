// androidqf - Android Quick Forensics
// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

const (
	acquireBrowserHistory = "Yes"
	skipBrowserHistory    = "No"
)

type BrowserHistory struct{}

type browserHistoryTarget struct {
	Browser    string
	Package    string
	Profile    string
	DevicePath string
}

type browserHistorySidecar struct {
	DevicePath  string `json:"device_path"`
	ArchivePath string `json:"archive_path"`
}

type browserHistoryDatabase struct {
	Browser     string                  `json:"browser"`
	Package     string                  `json:"package"`
	Profile     string                  `json:"profile"`
	DevicePath  string                  `json:"device_path"`
	ArchivePath string                  `json:"archive_path"`
	Sidecars    []browserHistorySidecar `json:"sidecars"`
}

type browserHistoryManifest struct {
	SchemaVersion     int                      `json:"schema_version"`
	GeneratedAt       time.Time                `json:"generated_at"`
	Status            string                   `json:"status"`
	AcquisitionMethod string                   `json:"acquisition_method"`
	Databases         []browserHistoryDatabase `json:"databases"`
}

// These locations are deliberately limited to paths backed by public parser
// fixtures or the historical MVT implementation. Additions require equivalent
// evidence; package-name guesses are not sufficient.
var browserHistoryTargets = []browserHistoryTarget{
	{"Chrome", "com.android.chrome", "Default", "/data/data/com.android.chrome/app_chrome/Default/History"},
	{"Brave", "com.brave.browser", "Default", "/data/data/com.brave.browser/app_chrome/Default/History"},
	{"Microsoft Edge", "com.microsoft.emmx", "Default", "/data/data/com.microsoft.emmx/app_chrome/Default/History"},
	{"Samsung Internet", "com.sec.android.app.sbrowser", "Default", "/data/data/com.sec.android.app.sbrowser/app_sbrowser/Default/History"},
}

func NewBrowserHistory() *BrowserHistory { return &BrowserHistory{} }

func (m *BrowserHistory) Name() string { return "browser_history" }

func ParseBrowserHistoryOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes":
		return acquireBrowserHistory, nil
	case "no":
		return skipBrowserHistory, nil
	}
	return "", fmt.Errorf("invalid -browser-history value %q (valid values: yes, no)", value)
}

func (m *BrowserHistory) Run(acq *acquisition.Acquisition, opts *Options) error {
	selection, err := resolveOption(opts, opts.BrowserHistory, "-browser-history (yes, no)", func() (string, error) {
		log.Info("Would you like to collect supported browser history databases? This requires existing root access.")
		prompt := promptui.Select{Label: "Browser history", Items: []string{skipBrowserHistory, acquireBrowserHistory}}
		_, selection, err := prompt.Run()
		return selection, err
	})
	if err != nil {
		return fmt.Errorf("failed to make selection for browser history: %w", err)
	}
	if selection == skipBrowserHistory {
		log.Info("Skipping browser history extraction...")
		return nil
	}

	manifest := browserHistoryManifest{
		SchemaVersion:     1,
		GeneratedAt:       time.Now().UTC(),
		Status:            "no_databases",
		AcquisitionMethod: "adb exec-out su -c cat",
		Databases:         []browserHistoryDatabase{},
	}
	if adb.Client == nil || !adb.Client.HasRoot() {
		manifest.Status = "root_unavailable"
		log.Warning("Browser history collection requires an already-functional su binary; no rooting was attempted.")
		return saveDataToAcquisition(acq, "browser_history/manifest.json", &manifest)
	}

	for _, target := range browserHistoryTargets {
		exists, err := adb.Client.FileExistsAsRoot(target.DevicePath)
		if err != nil {
			log.Warningf("Unable to check %s browser history: %v", target.Browser, err)
			continue
		}
		if !exists {
			continue
		}

		archivePath := path.Join("browser_history", target.Package, target.Profile, "History")
		if err := acq.PullRootToZipStaged(target.DevicePath, archivePath); err != nil {
			log.Warningf("Unable to collect %s browser history: %v", target.Browser, err)
			continue
		}
		database := browserHistoryDatabase{
			Browser: target.Browser, Package: target.Package, Profile: target.Profile,
			DevicePath: target.DevicePath, ArchivePath: archivePath,
			Sidecars: []browserHistorySidecar{},
		}
		for _, suffix := range []string{"-wal", "-shm"} {
			deviceSidecar := target.DevicePath + suffix
			exists, err := adb.Client.FileExistsAsRoot(deviceSidecar)
			if err != nil || !exists {
				continue
			}
			archiveSidecar := archivePath + suffix
			if err := acq.PullRootToZipStaged(deviceSidecar, archiveSidecar); err != nil {
				log.Warningf("Unable to collect %s sidecar %s: %v", target.Browser, suffix, err)
				continue
			}
			database.Sidecars = append(database.Sidecars, browserHistorySidecar{DevicePath: deviceSidecar, ArchivePath: archiveSidecar})
		}
		manifest.Databases = append(manifest.Databases, database)
	}

	if len(manifest.Databases) > 0 {
		manifest.Status = "collected"
	}
	return saveDataToAcquisition(acq, "browser_history/manifest.json", &manifest)
}
