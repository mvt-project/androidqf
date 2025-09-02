// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
	saveRuntime "github.com/botherder/go-savetime/runtime"
	"github.com/mvt-project/androidqf/log"
)

// EncryptedZipWriter provides streaming encrypted zip functionality
type EncryptedZipWriter struct {
	file       *os.File
	encWriter  io.WriteCloser
	zipWriter  *zip.Writer
	outputPath string
	closed     bool
}

// NewEncryptedZipWriter creates a new encrypted zip writer if key.txt exists
func NewEncryptedZipWriter(uuid string) (*EncryptedZipWriter, error) {
	cwd := saveRuntime.GetExecutableDirectory()
	keyFilePath := filepath.Join(cwd, "key.txt")

	// Check if key file exists
	if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("key.txt not found, encrypted streaming not available")
	}

	log.Info("Found age public key, using encrypted streaming mode.")

	// Read and parse public key
	publicKey, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %v", err)
	}
	publicKeyStr := strings.TrimSpace(string(publicKey))

	recipient, err := age.ParseX25519Recipient(publicKeyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key %q: %v", publicKeyStr, err)
	}

	// Create output file
	encFileName := fmt.Sprintf("%s.zip.age", uuid)
	outputPath := filepath.Join(cwd, encFileName)

	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}

	// Create encryption writer
	encWriter, err := age.Encrypt(file, recipient)
	if err != nil {
		file.Close()
		os.Remove(outputPath) // Clean up the created file
		return nil, fmt.Errorf("failed to create encrypted writer: %v", err)
	}

	// Create zip writer
	zipWriter := zip.NewWriter(encWriter)

	log.Infof("Started encrypted streaming to %s", outputPath)

	return &EncryptedZipWriter{
		file:       file,
		encWriter:  encWriter,
		zipWriter:  zipWriter,
		outputPath: outputPath,
		closed:     false,
	}, nil
}

// CreateFile creates a new file in the encrypted zip and returns a writer
func (ezw *EncryptedZipWriter) CreateFile(name string) (io.Writer, error) {
	if err := ezw.checkClosed(); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("file name cannot be empty")
	}

	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: time.Now(),
	}

	return ezw.zipWriter.CreateHeader(header)
}

// CreateFileFromReader copies data from a reader to a file in the encrypted zip
func (ezw *EncryptedZipWriter) CreateFileFromReader(name string, src io.Reader) error {
	if src == nil {
		return fmt.Errorf("source reader cannot be nil")
	}

	writer, err := ezw.CreateFile(name)
	if err != nil {
		return fmt.Errorf("failed to create file in zip: %v", err)
	}

	_, err = io.Copy(writer, src)
	if err != nil {
		return fmt.Errorf("failed to copy data to zip file: %v", err)
	}

	return nil
}

// CreateFileFromString creates a file with string content in the encrypted zip
func (ezw *EncryptedZipWriter) CreateFileFromString(name, content string) error {
	return ezw.CreateFileFromReader(name, strings.NewReader(content))
}

// CreateFileFromBytes creates a file with byte content in the encrypted zip
func (ezw *EncryptedZipWriter) CreateFileFromBytes(name string, content []byte) error {
	return ezw.CreateFileFromReader(name, bytes.NewReader(content))
}

// CreateFileFromPath reads a file from disk and adds it to the encrypted zip
func (ezw *EncryptedZipWriter) CreateFileFromPath(name, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %v", filePath, err)
	}
	defer file.Close()

	return ezw.CreateFileFromReader(name, file)
}

// Close finalizes and closes the encrypted zip
func (ezw *EncryptedZipWriter) Close() error {
	if ezw.closed {
		return nil
	}

	ezw.closed = true
	var lastErr error

	// Close zip writer first
	if err := ezw.zipWriter.Close(); err != nil {
		lastErr = fmt.Errorf("failed to close zip writer: %v", err)
	}

	// Close encryption writer
	if err := ezw.encWriter.Close(); err != nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("failed to close encryption writer: %v", err)
		}
	}

	// Close file
	if err := ezw.file.Close(); err != nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("failed to close output file: %v", err)
		}
	}

	if lastErr == nil {
		log.Infof("Encrypted archive created successfully at %s", ezw.outputPath)
	}
	return lastErr
}

// GetOutputPath returns the path to the encrypted zip file
func (ezw *EncryptedZipWriter) GetOutputPath() string {
	return ezw.outputPath
}

// IsClosed returns whether the writer has been closed
func (ezw *EncryptedZipWriter) IsClosed() bool {
	return ezw.closed
}

// checkClosed is a helper method to check if the writer is closed
func (ezw *EncryptedZipWriter) checkClosed() error {
	if ezw.closed {
		return fmt.Errorf("encrypted zip writer is closed")
	}
	return nil
}
