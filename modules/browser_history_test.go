package modules

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
)

func TestBrowserHistoryCollectsDatabaseAndManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}

	fakeADB := filepath.Join(t.TempDir(), "adb")
	script := `#!/bin/sh
case "$*" in
  *"id -u"*) printf '0' ;;
  *"if [ -f"*"com.android.chrome"*"History'"* ) printf '1' ;;
  *"if [ -f"*) printf '0' ;;
  *"cat --"*"com.android.chrome"*"History'"*) printf 'sqlite history' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(fakeADB, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile(fake adb) error = %v", err)
	}

	oldClient := adb.Client
	adb.Client = &adb.ADB{ExePath: fakeADB}
	defer func() { adb.Client = oldClient }()

	writer, err := acquisition.NewStreamingZipWriter("browser-history-test", t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	acq := &acquisition.Acquisition{
		ZipWriter: writer, StreamingMode: true,
		StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "", 1),
	}
	if err := NewBrowserHistory().Run(acq, &Options{BrowserHistory: acquireBrowserHistory}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	archive, err := zip.OpenReader(writer.GetOutputPath())
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	defer archive.Close()

	entries := make(map[string][]byte)
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("Open(%s) error = %v", file.Name, err)
		}
		entries[file.Name], err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("ReadAll(%s) error = %v", file.Name, err)
		}
	}

	dbPath := "browser_history/com.android.chrome/Default/History"
	if got := string(entries[dbPath]); got != "sqlite history" {
		t.Fatalf("database = %q, want sqlite history", got)
	}
	var manifest browserHistoryManifest
	if err := json.Unmarshal(entries["browser_history/manifest.json"], &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest) error = %v", err)
	}
	if manifest.Status != "collected" || len(manifest.Databases) != 1 {
		t.Fatalf("manifest = %+v, want one collected database", manifest)
	}
	if manifest.Databases[0].ArchivePath != dbPath {
		t.Fatalf("archive path = %q, want %q", manifest.Databases[0].ArchivePath, dbPath)
	}
}

func TestBrowserHistoryRecordsUnavailableRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}

	fakeADB := filepath.Join(t.TempDir(), "adb")
	if err := os.WriteFile(fakeADB, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("WriteFile(fake adb) error = %v", err)
	}
	oldClient := adb.Client
	adb.Client = &adb.ADB{ExePath: fakeADB}
	defer func() { adb.Client = oldClient }()

	writer, err := acquisition.NewStreamingZipWriter("browser-history-no-root", t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	acq := &acquisition.Acquisition{ZipWriter: writer, StreamingMode: true, StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "", 1)}
	if err := NewBrowserHistory().Run(acq, &Options{BrowserHistory: acquireBrowserHistory}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
