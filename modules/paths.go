// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type pullToWriter interface {
	PullToWriter(remotePath string, writer io.Writer) error
}

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

func createRootFile(root *os.Root, rel string) (*os.File, error) {
	localRel := filepath.FromSlash(rel)
	if !filepath.IsLocal(localRel) {
		return nil, fmt.Errorf("unsafe local path %q", rel)
	}

	if err := root.MkdirAll(filepath.Dir(localRel), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create destination folders for %q: %v", rel, err)
	}

	file, err := root.OpenFile(localRel, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file %q: %v", rel, err)
	}

	return file, nil
}

func streamDeviceChildToRoot(root *os.Root, puller pullToWriter, rel, devicePath string) error {
	file, err := createRootFile(root, rel)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := puller.PullToWriter(devicePath, file); err != nil {
		return err
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file %q: %v", rel, err)
	}

	return nil
}
