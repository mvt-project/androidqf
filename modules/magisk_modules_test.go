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

func TestMagiskModulesCollectsPropertiesAndState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}

	fakeADB := filepath.Join(t.TempDir(), "adb")
	script := `#!/bin/sh
case "$*" in
  *"id -u"*) printf '0' ;;
  *"for module in /data/adb/modules/"*) printf '/data/adb/modules/beta\0/data/adb/modules/alpha\0' ;;
  *"cat --"*"alpha/module.prop"*) printf 'id=alpha\nname=Alpha Module\nversion=1.0\n' ;;
  *"cat --"*"beta/module.prop"*) printf 'id=beta\nname=Beta Module\nversion=2.0\n' ;;
	*"wc -c <"*) printf '64' ;;
  *"beta/disable"*) printf '1' ;;
  *"module.prop"*) printf '1' ;;
  *"if [ -f"*) printf '0' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(fakeADB, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile(fake adb) error = %v", err)
	}

	oldClient := adb.Client
	adb.Client = &adb.ADB{ExePath: fakeADB}
	defer func() { adb.Client = oldClient }()

	writer, err := acquisition.NewStreamingZipWriter("magisk-modules-test", t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	acq := &acquisition.Acquisition{
		ZipWriter:       writer,
		StreamingMode:   true,
		StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "", 1),
	}
	if err := NewMagiskModules().Run(acq, &Options{MagiskModules: acquireMagiskModules}); err != nil {
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

	if got := string(entries["magisk_modules/0000/module.prop"]); got != "id=alpha\nname=Alpha Module\nversion=1.0\n" {
		t.Fatalf("alpha module.prop = %q", got)
	}
	if got := string(entries["magisk_modules/0001/module.prop"]); got != "id=beta\nname=Beta Module\nversion=2.0\n" {
		t.Fatalf("beta module.prop = %q", got)
	}

	var manifest magiskModulesManifest
	if err := json.Unmarshal(entries["magisk_modules/manifest.json"], &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest) error = %v", err)
	}
	if manifest.Status != "collected" || len(manifest.Modules) != 2 {
		t.Fatalf("manifest = %+v, want two collected modules", manifest)
	}
	if manifest.Modules[0].DirectoryName != "alpha" || manifest.Modules[0].ModulePropPath != "magisk_modules/0000/module.prop" {
		t.Fatalf("first module = %+v", manifest.Modules[0])
	}
	if len(manifest.Modules[0].StateFiles) != 0 || !manifest.Modules[0].StateFilesComplete {
		t.Fatalf("alpha state = %+v", manifest.Modules[0])
	}
	if len(manifest.Modules[1].StateFiles) != 1 || manifest.Modules[1].StateFiles[0] != "disable" {
		t.Fatalf("beta state files = %v, want disable", manifest.Modules[1].StateFiles)
	}
}

func TestMagiskModulesRecordsUnavailableRoot(t *testing.T) {
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

	writer, err := acquisition.NewStreamingZipWriter("magisk-modules-no-root", t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	acq := &acquisition.Acquisition{
		ZipWriter:       writer,
		StreamingMode:   true,
		StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "", 1),
	}
	if err := NewMagiskModules().Run(acq, &Options{MagiskModules: acquireMagiskModules}); err != nil {
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

	reader, err := archive.Open("magisk_modules/manifest.json")
	if err != nil {
		t.Fatalf("Open(manifest) error = %v", err)
	}
	defer reader.Close()
	var manifest magiskModulesManifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		t.Fatalf("Decode(manifest) error = %v", err)
	}
	if manifest.Status != "root_unavailable" || len(manifest.Modules) != 0 {
		t.Fatalf("manifest = %+v, want root_unavailable", manifest)
	}
}
