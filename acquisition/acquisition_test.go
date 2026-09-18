package acquisition

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		UUID:             "test-acquisition",
		ADBHostPublicKey: "AAAA-test-adb-public-key user@host",
		StoragePath:      zipWriter.GetOutputPath(),
		Started:          started,
		ZipWriter:        zipWriter,
		StreamingMode:    true,
		ModuleResults: []ModuleResult{{
			Name:      "files",
			Status:    "failed",
			Error:     "partial collection",
			Started:   started,
			Completed: started.Add(time.Second),
		}},
		logBuffer: bytes.NewBufferString("logged command\n"),
	}

	if err := acq.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if acq.Completed.IsZero() {
		t.Fatal("Complete() left Completed unset")
	}

	files := readZipFiles(t, filepath.Join(outputDir, "test-acquisition.zip"))
	if files["command.log"] != "logged command\n" {
		t.Fatalf("command.log = %q", files["command.log"])
	}
	if files["adb_host_key.pub"] != "AAAA-test-adb-public-key user@host\n" {
		t.Fatalf("adb_host_key.pub = %q", files["adb_host_key.pub"])
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
	if len(stored.ModuleResults) != 1 || stored.ModuleResults[0].Status != "failed" || stored.ModuleResults[0].Error != "partial collection" {
		t.Fatalf("module results = %+v", stored.ModuleResults)
	}
	if stored.ADBHostPublicKey != acq.ADBHostPublicKey {
		t.Fatalf("acquisition.json ADB host public key = %q, want %q", stored.ADBHostPublicKey, acq.ADBHostPublicKey)
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

	if err := acq.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if !acq.Completed.Equal(completed) {
		t.Fatalf("Complete() changed Completed from %s to %s", completed, acq.Completed)
	}
}

func TestCompleteReturnsArchiveWriteErrors(t *testing.T) {
	outputDir := t.TempDir()
	t.Chdir(outputDir)

	zipWriter, err := NewStreamingZipWriter("test-acquisition", outputDir)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	acq := &Acquisition{
		UUID:      "test-acquisition",
		ZipWriter: zipWriter,
	}
	if err := acq.Complete(); err == nil {
		t.Fatal("Complete() error = nil, want archive finalization error")
	}
}

type closeErrorWriter struct{}

func (closeErrorWriter) Write(p []byte) (int, error) { return len(p), nil }
func (closeErrorWriter) Close() error                { return errors.New("close failed") }

func TestCompleteReturnsArchiveFinalizationErrors(t *testing.T) {
	outputFile, err := os.CreateTemp(t.TempDir(), "archive-*.zip.age")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}

	var archive bytes.Buffer
	zipWriter := &StreamingZipWriter{
		file:      outputFile,
		encWriter: closeErrorWriter{},
		zipWriter: zip.NewWriter(&archive),
	}
	acq := &Acquisition{
		UUID:      "test-acquisition",
		ZipWriter: zipWriter,
	}

	err = acq.Complete()
	if err == nil || !strings.Contains(err.Error(), "failed to close archive") {
		t.Fatalf("Complete() error = %v, want archive close error", err)
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

func TestStageStreamToZipDoesNotCreateEntryForFailedProducer(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(fmt.Sprintf("encrypted=%v", encrypted), func(t *testing.T) {
			outputDir := t.TempDir()
			zipWriter, err := NewStreamingZipWriter("failed-stream", outputDir)
			if err != nil {
				t.Fatalf("NewStreamingZipWriter() error = %v", err)
			}
			zipWriter.encrypted = encrypted

			acq := &Acquisition{ZipWriter: zipWriter}
			err = acq.stageStreamToZip("backup.ab", func(writer io.Writer) error {
				if _, err := io.WriteString(writer, "partial evidence"); err != nil {
					return err
				}
				return errors.New("producer failed")
			})
			if err == nil {
				t.Fatal("stageStreamToZip() error = nil")
			}
			if err := zipWriter.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}

			reader, err := zip.OpenReader(zipWriter.GetOutputPath())
			if err != nil {
				t.Fatalf("zip.OpenReader() error = %v", err)
			}
			defer reader.Close()
			for _, file := range reader.File {
				if file.Name == "backup.ab" {
					t.Fatal("failed producer left backup.ab in archive")
				}
			}
		})
	}
}
