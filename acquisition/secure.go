// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/mvt-project/androidqf/log"
)

func createZipFile(sourceDir, zipPath string) error {
	sourceAbs, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve source directory: %v", err)
	}
	zipAbs, err := filepath.Abs(zipPath)
	if err != nil {
		return fmt.Errorf("failed to resolve ZIP path: %v", err)
	}

	zipFile, err := os.OpenFile(zipPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create ZIP file: %v", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	err = filepath.WalkDir(sourceDir, func(filePath string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirEntry.IsDir() {
			return nil
		}

		fileAbs, err := filepath.Abs(filePath)
		if err != nil {
			return err
		}
		if fileAbs == zipAbs {
			return nil
		}

		fileInfo, err := dirEntry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return nil
		}

		relPath, err := filepath.Rel(sourceAbs, fileAbs)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = zipWriter.Close()
		_ = zipFile.Close()
		return fmt.Errorf("failed to add directory to ZIP: %v", err)
	}

	if err := zipWriter.Close(); err != nil {
		_ = zipFile.Close()
		return fmt.Errorf("failed to close ZIP writer: %v", err)
	}
	if err := zipFile.Sync(); err != nil {
		_ = zipFile.Close()
		return fmt.Errorf("failed to sync ZIP file: %v", err)
	}
	if err := zipFile.Close(); err != nil {
		return fmt.Errorf("failed to close ZIP file: %v", err)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to verify ZIP file: %v", err)
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("failed to close ZIP verification reader: %v", err)
	}

	return nil
}

func (a *Acquisition) StoreSecurely() error {
	// In streaming mode, data is already encrypted during collection
	if a.StreamingMode {
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	keyFilePath, ok, err := findAgeKeyFile()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	log.Info("You provided an age public key, storing the acquisition securely.")

	zipFileName := fmt.Sprintf("%s.zip", a.UUID)
	zipFilePath := filepath.Join(cwd, zipFileName)

	log.Info("Compressing the acquisition folder. This might take a while...")

	err = createZipFile(a.StoragePath, zipFilePath)
	if err != nil {
		_ = os.Remove(zipFilePath)
		return err
	}

	log.Info("Encrypting the compressed archive. This might take a while...")

	publicKey, err := os.ReadFile(keyFilePath)
	if err != nil {
		return err
	}
	publicKeyStr := strings.TrimSpace(string(publicKey))

	recipient, err := age.ParseX25519Recipient(publicKeyStr)
	if err != nil {
		return fmt.Errorf("failed to parse public key %q: %v", publicKeyStr, err)
	}

	zipFile, err := os.Open(zipFilePath)
	if err != nil {
		return err
	}
	zipFileClosed := false
	defer func() {
		if !zipFileClosed {
			zipFile.Close()
		}
	}()

	encFileName := fmt.Sprintf("%s.age", zipFileName)
	encFilePath := filepath.Join(cwd, encFileName)
	tmpEncFilePath := encFilePath + ".tmp"
	encFile, err := os.OpenFile(tmpEncFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("unable to create encrypted file: %v", err)
	}
	defer os.Remove(tmpEncFilePath)

	w, err := age.Encrypt(encFile, recipient)
	if err != nil {
		encFile.Close()
		return fmt.Errorf("failed to create encrypted file: %v", err)
	}

	_, err = io.Copy(w, zipFile)
	if err != nil {
		w.Close()
		encFile.Close()
		return fmt.Errorf("failed to write to encrypted file: %v", err)
	}

	if err := w.Close(); err != nil {
		encFile.Close()
		return fmt.Errorf("failed to close encrypted file: %v", err)
	}
	if err := encFile.Sync(); err != nil {
		encFile.Close()
		return fmt.Errorf("failed to sync encrypted file: %v", err)
	}
	if err := encFile.Close(); err != nil {
		return fmt.Errorf("failed to close encrypted output file: %v", err)
	}
	if err := os.Rename(tmpEncFilePath, encFilePath); err != nil {
		return fmt.Errorf("failed to move encrypted file into place: %v", err)
	}

	log.Infof("Acquisition successfully encrypted at %s", encFilePath)

	// TODO: we should securely wipe the files.
	if err := zipFile.Close(); err != nil {
		return fmt.Errorf("failed to close the unencrypted compressed archive: %v", err)
	}
	zipFileClosed = true
	err = os.Remove(zipFilePath)
	if err != nil {
		return fmt.Errorf("failed to delete the unencrypted compressed archive: %v", err)
	}

	// Ensure log file is closed before removing the acquisition directory
	if a.closeLog != nil {
		defer a.closeLog()
	}

	err = os.RemoveAll(a.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to delete the original unencrypted acquisition folder: %v", err)
	}

	return nil
}
