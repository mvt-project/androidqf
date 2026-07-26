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
	"github.com/mvt-project/androidqf/log"
)

// StreamingZipWriter provides streaming zip functionality, optionally wrapped
// in age encryption when a public key is available.
type StreamingZipWriter struct {
	file       *os.File
	encWriter  io.WriteCloser
	zipWriter  *zip.Writer
	outputPath string
	encrypted  bool
	closed     bool
	hashes     []*zipHash
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

// NewStreamingZipWriter creates a streaming zip writer in outputDir. If key.txt
// exists, the zip stream is age-encrypted and written as .zip.age.
func NewStreamingZipWriter(uuid, outputDir string) (*StreamingZipWriter, error) {
	if outputDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		outputDir = cwd
	}

	stat, err := os.Stat(outputDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output folder: %v", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat output folder: %v", err)
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("output path exists and is not a folder")
	}

	keyFilePath, ok, err := findAgeKeyFile()
	if err != nil {
		return nil, err
	}

	fileName := fmt.Sprintf("%s.zip", uuid)
	if ok {
		fileName += ".age"
	}
	outputPath := filepath.Join(outputDir, fileName)

	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %v", err)
	}

	var encWriter io.WriteCloser
	var zipSink io.Writer = file
	if ok {
		log.Info("Found age public key, streaming to encrypted zip archive.")

		publicKey, err := os.ReadFile(keyFilePath)
		if err != nil {
			file.Close()
			os.Remove(outputPath)
			return nil, fmt.Errorf("failed to read public key: %v", err)
		}
		publicKeyStr := strings.TrimSpace(string(publicKey))

		recipient, err := age.ParseX25519Recipient(publicKeyStr)
		if err != nil {
			file.Close()
			os.Remove(outputPath)
			return nil, fmt.Errorf("failed to parse public key %q: %v", publicKeyStr, err)
		}

		encWriter, err = age.Encrypt(file, recipient)
		if err != nil {
			file.Close()
			os.Remove(outputPath)
			return nil, fmt.Errorf("failed to create encrypted writer: %v", err)
		}
		zipSink = encWriter
	} else {
		log.Info("No age public key found, streaming to unencrypted zip archive.")
	}

	zipWriter := zip.NewWriter(zipSink)

	log.Infof("Started streaming to %s", outputPath)

	return &StreamingZipWriter{
		file:       file,
		encWriter:  encWriter,
		zipWriter:  zipWriter,
		outputPath: outputPath,
		encrypted:  ok,
		closed:     false,
	}, nil
}

// CreateFile creates a new file in the zip and returns a writer
func (ezw *StreamingZipWriter) CreateFile(name string) (io.Writer, error) {
	return ezw.createFile(name, true)
}

func (ezw *StreamingZipWriter) createFile(name string, trackHash bool) (io.Writer, error) {
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

// CreateFileFromReader copies data from a reader to a file in the zip
func (ezw *StreamingZipWriter) CreateFileFromReader(name string, src io.Reader) error {
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

// CreateHashList adds hashes.csv to the zip with SHA-256 hashes of
// the plaintext zip entries written so far.
func (ezw *StreamingZipWriter) CreateHashList() error {
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

// CreateFileFromString creates a file with string content in the zip
func (ezw *StreamingZipWriter) CreateFileFromString(name, content string) error {
	return ezw.CreateFileFromReader(name, strings.NewReader(content))
}

// CreateFileFromBytes creates a file with byte content in the zip
func (ezw *StreamingZipWriter) CreateFileFromBytes(name string, content []byte) error {
	return ezw.CreateFileFromReader(name, bytes.NewReader(content))
}

// CreateFileFromPath reads a file from disk and adds it to the zip
func (ezw *StreamingZipWriter) CreateFileFromPath(name, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %v", filePath, err)
	}
	defer file.Close()

	return ezw.CreateFileFromReader(name, file)
}

// Close finalizes and closes the zip
func (ezw *StreamingZipWriter) Close() error {
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
		log.Infof("Archive created successfully at %s", ezw.outputPath)
	}
	return lastErr
}

// GetOutputPath returns the path to the zip file
func (ezw *StreamingZipWriter) GetOutputPath() string {
	return ezw.outputPath
}

func (ezw *StreamingZipWriter) IsEncrypted() bool {
	return ezw.encrypted
}

// IsClosed returns whether the writer has been closed
func (ezw *StreamingZipWriter) IsClosed() bool {
	return ezw.closed
}

// checkClosed is a helper method to check if the writer is closed
func (ezw *StreamingZipWriter) checkClosed() error {
	if ezw.closed {
		return fmt.Errorf("streaming zip writer is closed")
	}
	return nil
}
