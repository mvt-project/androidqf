package acquisition

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreInfoSetsCompletedTimestamp(t *testing.T) {
	acq := &Acquisition{
		UUID:        "test-acquisition",
		StoragePath: t.TempDir(),
	}

	if err := acq.StoreInfo(); err != nil {
		t.Fatalf("StoreInfo() error = %v", err)
	}

	if acq.Completed.IsZero() {
		t.Fatal("StoreInfo() left Completed unset")
	}

	info, err := os.ReadFile(filepath.Join(acq.StoragePath, "acquisition.json"))
	if err != nil {
		t.Fatalf("ReadFile(acquisition.json) error = %v", err)
	}

	var stored Acquisition
	if err := json.Unmarshal(info, &stored); err != nil {
		t.Fatalf("json.Unmarshal(acquisition.json) error = %v", err)
	}
	if stored.Completed.IsZero() {
		t.Fatal("acquisition.json contains a zero completed timestamp")
	}
}

func TestCompleteDoesNotOverwriteExistingCompletedTimestamp(t *testing.T) {
	acq := &Acquisition{
		UUID:        "test-acquisition",
		StoragePath: t.TempDir(),
	}

	if err := acq.StoreInfo(); err != nil {
		t.Fatalf("StoreInfo() error = %v", err)
	}
	completed := acq.Completed

	acq.Complete()

	if !acq.Completed.Equal(completed) {
		t.Fatalf("Complete() changed Completed from %s to %s", completed, acq.Completed)
	}
}

func TestStoreSecurelyUsesCurrentWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	writeTestAgeKey(t, cwd)

	storagePath := filepath.Join(cwd, "test-acquisition")
	if err := os.Mkdir(storagePath, 0o755); err != nil {
		t.Fatalf("Mkdir(storagePath) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(storagePath, "data.txt"), []byte("evidence"), 0o600); err != nil {
		t.Fatalf("WriteFile(data.txt) error = %v", err)
	}

	acq := &Acquisition{
		UUID:        "test-acquisition",
		StoragePath: storagePath,
	}

	if err := acq.StoreSecurely(); err != nil {
		t.Fatalf("StoreSecurely() error = %v", err)
	}

	wantPath := filepath.Join(cwd, "test-acquisition.zip.age")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("Stat(encrypted output) error = %v", err)
	}
	if _, err := os.Stat(storagePath); !os.IsNotExist(err) {
		t.Fatalf("storage path still exists or returned unexpected error: %v", err)
	}
}

func TestCreateZipFileCreatesReadableArchive(t *testing.T) {
	sourcePath := t.TempDir()
	if err := os.Mkdir(filepath.Join(sourcePath, "nested"), 0o755); err != nil {
		t.Fatalf("Mkdir(nested) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "nested", "data.txt"), []byte("evidence"), 0o600); err != nil {
		t.Fatalf("WriteFile(data.txt) error = %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "acquisition.zip")
	if err := createZipFile(sourcePath, zipPath); err != nil {
		t.Fatalf("createZipFile() error = %v", err)
	}

	files := readZipFiles(t, zipPath)
	if files["nested/data.txt"] != "evidence" {
		t.Fatalf("nested/data.txt = %q, want evidence", files["nested/data.txt"])
	}
}

func TestCreateZipFileSkipsArchiveInsideSourceDirectory(t *testing.T) {
	sourcePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourcePath, "data.txt"), []byte("evidence"), 0o600); err != nil {
		t.Fatalf("WriteFile(data.txt) error = %v", err)
	}

	zipPath := filepath.Join(sourcePath, "acquisition.zip")
	if err := createZipFile(sourcePath, zipPath); err != nil {
		t.Fatalf("createZipFile() error = %v", err)
	}

	files := readZipFiles(t, zipPath)
	if files["data.txt"] != "evidence" {
		t.Fatalf("data.txt = %q, want evidence", files["data.txt"])
	}
	if _, ok := files["acquisition.zip"]; ok {
		t.Fatal("archive contains itself")
	}
}

func readZipFiles(t *testing.T, zipPath string) map[string]string {
	t.Helper()

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("OpenReader(%q) error = %v", zipPath, err)
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
