// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

func relativeDeviceChild(deviceRoot, devicePath string) (string, error) {
	if deviceRoot == "" {
		return "", fmt.Errorf("device root cannot be empty")
	}
	if strings.ContainsRune(devicePath, 0) {
		return "", fmt.Errorf("unsafe device path %q", devicePath)
	}

	root := path.Clean(deviceRoot)
	child := path.Clean(devicePath)
	if child == root {
		return "", fmt.Errorf("device path %q is the root path %q", devicePath, deviceRoot)
	}

	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, "/") {
		rootPrefix += "/"
	}
	if !strings.HasPrefix(child, rootPrefix) {
		return "", fmt.Errorf("device path %q is outside %q", devicePath, deviceRoot)
	}

	rel := strings.TrimPrefix(child, rootPrefix)
	localRel := filepath.FromSlash(rel)
	if !filepath.IsLocal(localRel) {
		return "", fmt.Errorf("unsafe device path %q", devicePath)
	}

	return rel, nil
}
