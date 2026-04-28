package acquisition

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

func TestCreateHashListTracksPlaintextZipEntries(t *testing.T) {
	var archive bytes.Buffer
	ezw := &EncryptedZipWriter{
		zipWriter: zip.NewWriter(&archive),
	}

	if err := ezw.CreateFileFromString("first.txt", "first content"); err != nil {
		t.Fatalf("CreateFileFromString() error = %v", err)
	}

	writer, err := ezw.CreateFile("stream.bin")
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if _, err := writer.Write([]byte("streamed ")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := writer.Write([]byte("content")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := ezw.CreateHashList(); err != nil {
		t.Fatalf("CreateHashList() error = %v", err)
	}
	if err := ezw.zipWriter.Close(); err != nil {
		t.Fatalf("zip Close() error = %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}

	files := make(map[string]string)
	for _, file := range reader.File {
		readCloser, err := file.Open()
		if err != nil {
			t.Fatalf("Open(%q) error = %v", file.Name, err)
		}
		content, err := io.ReadAll(readCloser)
		readCloser.Close()
		if err != nil {
			t.Fatalf("ReadAll(%q) error = %v", file.Name, err)
		}
		files[file.Name] = string(content)
	}

	records, err := csv.NewReader(strings.NewReader(files["hashes.csv"])).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll(hashes.csv) error = %v", err)
	}

	gotHashes := make(map[string]string)
	for _, record := range records {
		if len(record) != 2 {
			t.Fatalf("hash record has %d fields, want 2: %#v", len(record), record)
		}
		gotHashes[record[0]] = record[1]
	}

	wantHashes := map[string]string{
		"first.txt":  sha256Hex("first content"),
		"stream.bin": sha256Hex("streamed content"),
	}
	if len(gotHashes) != len(wantHashes) {
		t.Fatalf("got %d hash records, want %d: %#v", len(gotHashes), len(wantHashes), gotHashes)
	}
	for name, wantHash := range wantHashes {
		if gotHashes[name] != wantHash {
			t.Fatalf("hash for %q = %q, want %q", name, gotHashes[name], wantHash)
		}
	}
	if _, ok := gotHashes["hashes.csv"]; ok {
		t.Fatal("hashes.csv should not include a hash record for itself")
	}
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
