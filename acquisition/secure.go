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
	saveRuntime "github.com/botherder/go-savetime/runtime"
	"github.com/mvt-project/androidqf/log"
	"github.com/spf13/afero"
)

// createZipFromFs creates a zip file from any afero filesystem
func createZipFromFs(fs afero.Fs, sourceDir string, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Walk the filesystem and add all files to the zip
	err := afero.Walk(fs, sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path from sourceDir
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Create the file header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath) // Ensure forward slashes for zip compatibility
		header.Method = zip.Deflate

		// Create the file in the zip
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// Read the file content from the filesystem
		file, err := fs.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Copy file content to zip
		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to walk filesystem: %v", err)
	}

	return zipWriter.Close()
}

// createZipFromDisk creates a zip file from disk (legacy approach)
func createZipFromDisk(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create ZIP file: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Use AddFS to add the entire directory
	fsys := os.DirFS(sourceDir)
	err = zipWriter.AddFS(fsys)
	if err != nil {
		return fmt.Errorf("failed to add directory to ZIP: %v", err)
	}

	return nil
}

func (a *Acquisition) StoreSecurely() error {
	// Only proceed if key file is present
	if !a.KeyFilePresent {
		return nil
	}

	log.Info("Age public key detected, storing the acquisition securely.")

	cwd := saveRuntime.GetExecutableDirectory()
	keyFilePath := filepath.Join(cwd, "key.txt")

	// Read the public key
	publicKey, err := os.ReadFile(keyFilePath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %v", err)
	}
	publicKeyStr := strings.TrimSpace(string(publicKey))

	// Parse the age recipient
	recipient, err := age.ParseX25519Recipient(publicKeyStr)
	if err != nil {
		return fmt.Errorf("failed to parse public key %q: %v", publicKeyStr, err)
	}

	// Create the encrypted file
	encFileName := fmt.Sprintf("%s.age", a.UUID)
	encFilePath := filepath.Join(cwd, encFileName)
	encFile, err := os.OpenFile(encFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("unable to create encrypted file: %v", err)
	}
	defer encFile.Close()

	// Create age encryptor
	ageWriter, err := age.Encrypt(encFile, recipient)
	if err != nil {
		return fmt.Errorf("failed to create age encryptor: %v", err)
	}

	if a.UseMemoryFs {
		log.Info("Compressing and encrypting acquisition data from memory. This might take a while...")

		// Create zip directly from memory filesystem and encrypt it
		err = createZipFromFs(a.Fs, a.StoragePath, ageWriter)
		if err != nil {
			return fmt.Errorf("failed to create encrypted zip from memory: %v", err)
		}
	} else {
		log.Info("Compressing the acquisition folder. This might take a while...")

		// Create temporary zip file on disk first
		zipFileName := fmt.Sprintf("%s.zip", a.UUID)
		zipFilePath := filepath.Join(cwd, zipFileName)

		err := createZipFromDisk(a.StoragePath, zipFilePath)
		if err != nil {
			return fmt.Errorf("failed to create zip file: %v", err)
		}

		log.Info("Encrypting the compressed archive. This might take a while...")

		// Read the zip file and encrypt it
		zipFile, err := os.Open(zipFilePath)
		if err != nil {
			return fmt.Errorf("failed to open zip file: %v", err)
		}

		_, err = io.Copy(ageWriter, zipFile)
		zipFile.Close()

		if err != nil {
			return fmt.Errorf("failed to encrypt zip file: %v", err)
		}

		// Remove the temporary unencrypted zip file
		err = os.Remove(zipFilePath)
		if err != nil {
			log.Warningf("Failed to remove temporary zip file: %v", err)
		}
	}

	// Close the age writer
	if err := ageWriter.Close(); err != nil {
		return fmt.Errorf("failed to close encrypted file: %v", err)
	}

	log.Infof("Acquisition successfully encrypted at %s", encFilePath)

	// Clean up based on filesystem type
	if a.UseMemoryFs {
		log.Info("Clearing in-memory data...")
		// For memory filesystem, we just need to ensure the log is closed
		// The memory will be freed when the filesystem is dereferenced
	} else {
		log.Info("Removing unencrypted acquisition folder...")
		// Ensure log file is closed before removing the acquisition directory
		if a.closeLog != nil {
			a.closeLog()
			a.closeLog = nil // Prevent double-close
		}

		// Remove the original unencrypted folder
		err = os.RemoveAll(a.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to delete the original unencrypted acquisition folder: %v", err)
		}
	}

	return nil
}
