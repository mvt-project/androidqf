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

		assetFile, err := os.OpenFile(assetPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o755)
		if err != nil {
			// Asset file already exists. Do not try overwriting
			if errors.Is(err, os.ErrExist) {
				continue
			} else {
				return err
			}
		}
		defer assetFile.Close()

		_, err = assetFile.Write(asset.Data)
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
