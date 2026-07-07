// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"os"
	"path/filepath"
)

const keyFileName = "key.txt"

func findAgeKeyFile() (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}
	if keyPath, ok := findAgeKeyFileInDirs(cwd, ""); ok {
		return keyPath, true, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", false, err
	}

	keyPath, ok := findAgeKeyFileInDirs(cwd, filepath.Dir(exe))
	return keyPath, ok, nil
}

func findAgeKeyFileInDirs(cwd, executableDir string) (string, bool) {
	for _, dir := range []string{cwd, executableDir} {
		if dir == "" {
			continue
		}

		keyPath := filepath.Join(dir, keyFileName)
		if _, err := os.Stat(keyPath); err == nil {
			return keyPath, true
		}
	}

	return "", false
}
