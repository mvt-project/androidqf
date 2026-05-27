// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
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
	hashes     []*zipHash
	encrypted  bool
}

type zipHash struct {
	name   string
	hasher hash.Hash
}

type hashingWriter struct {
	writer io.Writer
	hasher hash.Hash
}

func (hw *hashingWriter) Write(p []byte) (int, error) {
	n, err := hw.writer.Write(p)
	if n > 0 {
		_, _ = hw.hasher.Write(p[:n])
	}
	return n, err
}

// NewEncryptedZipWriter creates a new streaming zip writer.
// If key.txt exists next to the executable, the zip stream is encrypted with
// age and written as <uuid>.zip.age. If key.txt is missing, the zip stream is
// written unencrypted as <uuid>.zip.
func NewEncryptedZipWriter(uuid string) (*EncryptedZipWriter, error) {
	cwd := saveRuntime.GetExecutableDirectory()
	keyFilePath := filepath.Join(cwd, "key.txt")

	if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
		log.Info("No age public key found, using unencrypted zip streaming mode.")
		return newPlainZipWriter(cwd, uuid)
	} else if err != nil {
		return nil, fmt.Errorf("failed to check key.txt: %v", err)
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
		encrypted:  true,
	}, nil
}

func newPlainZipWriter(cwd, uuid string) (*EncryptedZipWriter, error) {
	zipFileName := fmt.Sprintf("%s.zip", uuid)
	outputPath := filepath.Join(cwd, zipFileName)

	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}

	zipWriter := zip.NewWriter(file)

	log.Infof("Started unencrypted zip streaming to %s", outputPath)

	return &EncryptedZipWriter{
		file:       file,
		zipWriter:  zipWriter,
		outputPath: outputPath,
		closed:     false,
		encrypted:  false,
	}, nil
}

// CreateFile creates a new file in the encrypted zip and returns a writer
func (ezw *EncryptedZipWriter) CreateFile(name string) (io.Writer, error) {
	return ezw.createFile(name, true)
}

func (ezw *EncryptedZipWriter) createFile(name string, trackHash bool) (io.Writer, error) {
	if err := ezw.checkClosed(); err != nil {
		return nil, err
	}

	if err := validateZipEntryName(name); err != nil {
		return nil, err
	}

	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: time.Now(),
	}

	writer, err := ezw.zipWriter.CreateHeader(header)
	if err != nil {
		return nil, err
	}

	if !trackHash {
		return writer, nil
	}

	zipHash := &zipHash{
		name:   name,
		hasher: sha256.New(),
	}
	ezw.hashes = append(ezw.hashes, zipHash)

	return &hashingWriter{
		writer: writer,
		hasher: zipHash.hasher,
	}, nil
}

func validateZipEntryName(name string) error {
	if name == "" {
		return fmt.Errorf("file name cannot be empty")
	}
	if strings.ContainsAny(name, "\\\x00") {
		return fmt.Errorf("unsafe zip entry name: %q", name)
	}
	if path.IsAbs(name) {
		return fmt.Errorf("unsafe zip entry name: %q", name)
	}

	first, _, _ := strings.Cut(name, "/")
	if len(first) >= 2 && first[1] == ':' {
		return fmt.Errorf("unsafe zip entry name: %q", name)
	}

	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return fmt.Errorf("unsafe zip entry name: %q", name)
		}
	}
	if path.Clean(name) == "." {
		return fmt.Errorf("unsafe zip entry name: %q", name)
	}

	return nil
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

// CreateHashList adds hashes.csv to the encrypted zip with SHA-256 hashes of
// the plaintext zip entries written so far.
func (ezw *EncryptedZipWriter) CreateHashList() error {
	if err := ezw.checkClosed(); err != nil {
		return err
	}

	var buffer bytes.Buffer
	csvWriter := csv.NewWriter(&buffer)
	for _, zipHash := range ezw.hashes {
		if err := csvWriter.Write([]string{
			zipHash.name,
			hex.EncodeToString(zipHash.hasher.Sum(nil)),
		}); err != nil {
			return fmt.Errorf("failed to write hash entry for %q: %v", zipHash.name, err)
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("failed to create hash list: %v", err)
	}

	writer, err := ezw.createFile("hashes.csv", false)
	if err != nil {
		return fmt.Errorf("failed to create hashes.csv in zip: %v", err)
	}
	if _, err := writer.Write(buffer.Bytes()); err != nil {
		return fmt.Errorf("failed to write hashes.csv to zip: %v", err)
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

// Close finalizes and closes the streaming zip
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

	if ezw.encWriter != nil {
		// Close encryption writer
		if err := ezw.encWriter.Close(); err != nil {
			if lastErr == nil {
				lastErr = fmt.Errorf("failed to close encryption writer: %v", err)
			}
		}
	}

	// Close file
	if err := ezw.file.Close(); err != nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("failed to close output file: %v", err)
		}
	}

	if lastErr == nil {
		if ezw.encrypted {
			log.Infof("Encrypted archive created successfully at %s", ezw.outputPath)
		} else {
			log.Infof("Unencrypted zip archive created successfully at %s", ezw.outputPath)
		}
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

// IsEncrypted returns whether the zip stream is encrypted with age.
func (ezw *EncryptedZipWriter) IsEncrypted() bool {
	return ezw.encrypted
}

// checkClosed is a helper method to check if the writer is closed
func (ezw *EncryptedZipWriter) checkClosed() error {
	if ezw.closed {
		return fmt.Errorf("zip writer is closed")
	}
	return nil
}
