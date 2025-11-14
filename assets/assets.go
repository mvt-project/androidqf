// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package assets

import (
	"embed"
	"errors"
	"os"
	"path/filepath"

	saveRuntime "github.com/botherder/go-savetime/runtime"
)

//go:embed collector_*
var Collector embed.FS

type Asset struct {
	Name string
	Data []byte
}

// DeployAssets is used to retrieve the embedded adb binaries and store them.
func DeployAssets() error {
	cwd := saveRuntime.GetExecutableDirectory()

	for _, asset := range getAssets() {
		assetPath := filepath.Join(cwd, asset.Name)

		// If the file already exists, skip it. This avoids failing when adb
		// is already deployed or in use by another process.
		if _, err := os.Stat(assetPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			// Can't determine file existence (e.g., permission error); skip deploying this asset.
			continue
		}

		// Try to create the asset file. If creation fails (for example because
		// the file was created between the Stat and OpenFile calls, or because
		// the file is locked by another process), skip the asset instead of failing.
		assetFile, err := os.OpenFile(assetPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o755)
		if err != nil {
			// If the file exists now, just continue; otherwise skip this asset.
			if errors.Is(err, os.ErrExist) {
				continue
			}
			// Could be locked or another transient error — do not fail the whole deployment.
			continue
		}

		// Write and close immediately (avoid defer in a loop).
		_, err = assetFile.Write(asset.Data)
		assetFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// Remove assets from the local disk
func CleanAssets() error {
	cwd := saveRuntime.GetExecutableDirectory()

	for _, asset := range getAssets() {
		assetPath := filepath.Join(cwd, asset.Name)
		err := os.Remove(assetPath)
		if err != nil {
			return err
		}
	}

	return nil
}
