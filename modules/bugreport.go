// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
)

type Bugreport struct {
	StoragePath       string
	OldBugreportsPath string
}

func NewBugreport() *Bugreport {
	return &Bugreport{}
}

var oldBugreportDeviceRoots = []string{
	"/bugreports/",
	"/data/user_de/0/com.android.shell/files/",
}

type oldBugreportFile struct {
	DevicePath  string
	ArchiveName string
}

func (b *Bugreport) Name() string {
	return "bugreport"
}

func (b *Bugreport) InitStorage(storagePath string) error {
	b.StoragePath = storagePath
	b.OldBugreportsPath = filepath.Join(storagePath, "old_bugreports")

	if storagePath != "" {
		err := os.Mkdir(b.OldBugreportsPath, 0o755)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to create old bugreports folder: %v", err)
		}
	}

	return nil
}

func (b *Bugreport) Run(acq *acquisition.Acquisition, fast bool) error {
	if err := b.pullOldBugreports(acq); err != nil {
		log.Errorf("Failed to collect old bugreports: %v\n", err)
	}

	log.Info(
		"Generating a bugreport for the device...",
	)

	if acq.StreamingMode && acq.EncryptedWriter != nil {
		// Streaming mode: stream bugreport directly to encrypted zip without temp files
		err := acq.StreamBugreportToZip("bugreport.zip")
		if err != nil {
			return fmt.Errorf("failed to stream bugreport to encrypted archive: %v", err)
		}
	} else {
		// Traditional mode: write directly into acquisition dir.
		bugreportPath := filepath.Join(b.StoragePath, "bugreport.zip")
		err := adb.Client.Bugreport(bugreportPath)
		if err != nil {
			log.Debugf("Impossible to generate bugreport: %v", err)
			return err
		}
	}

	log.Debug("Bugreport completed!")

	return nil
}

func (b *Bugreport) pullOldBugreports(acq *acquisition.Acquisition) error {
	files := b.listOldBugreports()
	if len(files) == 0 {
		log.Debug("No old bugreports found on device.")
		return nil
	}

	log.Infof("Collecting %d old bugreport files from the device...", len(files))

	streaming := acq.StreamingMode && acq.EncryptedWriter != nil
	var localRoot *os.Root
	var puller *acquisition.StreamingPuller
	if !streaming {
		var err error
		localRoot, err = os.OpenRoot(b.OldBugreportsPath)
		if err != nil {
			return fmt.Errorf("failed to open old bugreports output root: %v", err)
		}
		defer localRoot.Close()
		puller = acquisition.NewStreamingPuller(adb.Client.ExePath, adb.Client.Serial, 100)
	}

	for _, file := range files {
		if streaming {
			zipPath := path.Join("old_bugreports", file.ArchiveName)

			writer, err := acq.EncryptedWriter.CreateFile(zipPath)
			if err != nil {
				log.Errorf("Failed to create zip entry for old bugreport %s: %v\n", file.DevicePath, err)
				continue
			}

			err = acq.StreamingPuller.PullToWriter(file.DevicePath, writer)
			if err != nil {
				log.Errorf("Failed to stream old bugreport %s: %v\n", file.DevicePath, err)
				continue
			}

			log.Debugf("Streamed old bugreport %s directly to encrypted archive as %s", file.DevicePath, zipPath)
		} else {
			if err := streamDeviceChildToRoot(localRoot, puller, file.ArchiveName, file.DevicePath); err != nil {
				log.Errorf("Failed to pull old bugreport %s: %v\n", file.DevicePath, err)
				continue
			}
		}
	}

	return nil
}

func (b *Bugreport) listOldBugreports() []oldBugreportFile {
	var candidates []string
	for _, root := range oldBugreportDeviceRoots {
		files, err := adb.Client.ListFiles(root, true)
		if err != nil {
			log.Debugf("Failed to list old bugreports in %s: %v", root, err)
			continue
		}
		candidates = append(candidates, files...)
	}

	return uniqueOldBugreportFiles(candidates)
}

func uniqueOldBugreportFiles(devicePaths []string) []oldBugreportFile {
	files := make([]oldBugreportFile, 0, len(devicePaths))
	seen := make(map[string]struct{}, len(devicePaths))

	for _, devicePath := range devicePaths {
		archiveName, ok := oldBugreportArchiveName(devicePath)
		if !ok {
			continue
		}
		if _, exists := seen[archiveName]; exists {
			continue
		}
		seen[archiveName] = struct{}{}
		files = append(files, oldBugreportFile{
			DevicePath:  devicePath,
			ArchiveName: archiveName,
		})
	}

	return files
}

func oldBugreportArchiveName(devicePath string) (string, bool) {
	if strings.ContainsRune(devicePath, 0) {
		return "", false
	}

	name := path.Base(strings.TrimSpace(devicePath))
	lowerName := strings.ToLower(name)
	if name == "." || name == "/" || lowerName == "bugreports" {
		return "", false
	}
	if lowerName != "bugreport.zip" && !strings.HasPrefix(lowerName, "bugreport-") {
		return "", false
	}
	if !filepath.IsLocal(filepath.FromSlash(name)) {
		return "", false
	}

	return name, true
}
