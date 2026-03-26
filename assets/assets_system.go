// androidqf - Android Quick Forensics
// Copyright (c) 2026 kpcyrd.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

//go:build unbundle

package assets

import (
	"os"
	"path/filepath"
)

// Read a specific collector binary from filesystem
func ReadCollectorFile(collectorName string) ([]byte, error) {
	path := filepath.Join("/usr/lib/androidqf/android-collector", collectorName)
	return os.ReadFile(path)
}

// Assets are expected to be installed by package manager
func DeployAssets() error {
	return nil
}

// No assets to clean up
func CleanAssets() error {
	return nil
}
