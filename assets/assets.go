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
)

//go:embed collector_*
var Collector embed.FS

type Asset struct {
	Name string
	Data []byte
}

// DeployAssetsToDir extracts the embedded adb binaries into the given directory.
// If a file already exists there it is silently skipped, so calling this
// function more than once (or concurrently) is safe.
func DeployAssetsToDir(dir string) error {
	for _, asset := range getAssets() {
		assetPath := filepath.Join(dir, asset.Name)

		// Already present – skip without error.
		if _, err := os.Stat(assetPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			// Permission or other stat error – skip this asset rather than abort.
			continue
		}

		// O_EXCL ensures we don't clobber a file created between Stat and here.
		f, err := os.OpenFile(assetPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o755)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			// Transient error (e.g. locked) – skip rather than abort.
			continue
		}

		_, writeErr := f.Write(asset.Data)
		f.Close()
		if writeErr != nil {
			return writeErr
		}
	}

	return nil
}
