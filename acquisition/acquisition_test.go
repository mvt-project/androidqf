package acquisition

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompleteWritesMetadataToStreamingZip(t *testing.T) {
	outputDir := t.TempDir()
	t.Chdir(outputDir)

	zipWriter, err := NewStreamingZipWriter("test-acquisition", outputDir)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}

	started := time.Now().UTC()
	acq := &Acquisition{
		UUID:          "test-acquisition",
		StoragePath:   zipWriter.GetOutputPath(),
		Started:       started,
		ZipWriter:     zipWriter,
		StreamingMode: true,
		logBuffer:     bytes.NewBufferString("logged command\n"),
	}

	acq.Complete()

	if acq.Completed.IsZero() {
		t.Fatal("Complete() left Completed unset")
	}

	files := readZipFiles(t, filepath.Join(outputDir, "test-acquisition.zip"))
	if files["command.log"] != "logged command\n" {
		t.Fatalf("command.log = %q", files["command.log"])
	}
	if _, ok := files["hashes.csv"]; !ok {
		t.Fatal("hashes.csv missing from archive")
	}

	var stored Acquisition
	if err := json.Unmarshal([]byte(files["acquisition.json"]), &stored); err != nil {
		t.Fatalf("json.Unmarshal(acquisition.json) error = %v", err)
	}
	if stored.Completed.IsZero() {
		t.Fatal("acquisition.json contains a zero completed timestamp")
	}
}

func TestCompleteDoesNotOverwriteExistingCompletedTimestamp(t *testing.T) {
	outputDir := t.TempDir()
	t.Chdir(outputDir)

	zipWriter, err := NewStreamingZipWriter("test-acquisition", outputDir)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}

	completed := time.Now().UTC().Add(-time.Hour)
	acq := &Acquisition{
		UUID:          "test-acquisition",
		StoragePath:   zipWriter.GetOutputPath(),
		Started:       completed.Add(-time.Hour),
		Completed:     completed,
		ZipWriter:     zipWriter,
		StreamingMode: true,
	}

	acq.Complete()

	if !acq.Completed.Equal(completed) {
		t.Fatalf("Complete() changed Completed from %s to %s", completed, acq.Completed)
	}
}

func readZipFiles(t *testing.T, archivePath string) map[string]string {
	t.Helper()

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("zip.OpenReader(%q) error = %v", archivePath, err)
	}
	defer reader.Close()

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

	return files
}

func TestNewStreamingZipWriterWithoutKeyCreatesPlainZip(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)

	ezw, err := NewStreamingZipWriter("test-acquisition", cwd)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	defer os.Remove(ezw.GetOutputPath())

	if ezw.IsEncrypted() {
		t.Fatal("writer is encrypted without key.txt")
	}
	if err := ezw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	wantPath := filepath.Join(cwd, "test-acquisition.zip")
	if ezw.GetOutputPath() != wantPath {
		t.Fatalf("output path = %q, want %q", ezw.GetOutputPath(), wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("Stat(output) error = %v", err)
	}
}
