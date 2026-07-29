package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessFileStreamsCompleteHashes(t *testing.T) {
	content := []byte("forensic content")
	filePath := filepath.Join(t.TempDir(), "evidence.bin")
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	got := processFile(filePath, info, true)
	want := sha256.Sum256(content)
	if got.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("SHA256 = %q, want %q", got.SHA256, hex.EncodeToString(want[:]))
	}
	if got.Error != "" {
		t.Fatalf("Error = %q", got.Error)
	}
}

func TestProcessFileHashesEmptyFiles(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(filePath, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	got := processFile(filePath, info, true)
	want := sha256.Sum256(nil)
	if got.SHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("SHA256 = %q, want %q", got.SHA256, hex.EncodeToString(want[:]))
	}
}

func TestIsPathWithin(t *testing.T) {
	if !isPathWithin("/proc/1/stat", "/proc") {
		t.Fatal("/proc/1/stat should be within /proc")
	}
	if isPathWithin("/proc-backup/file", "/proc") {
		t.Fatal("/proc-backup/file should not be within /proc")
	}
}
