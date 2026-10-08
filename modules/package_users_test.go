package modules

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
)

func TestPackagesArchivesAccessibleUserAndDeniedUserOutcome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX fake ADB executable")
	}
	fakeADB := filepath.Join(t.TempDir(), "adb")
	script := `#!/bin/sh
case "$*" in
  "shell pm list users") printf 'Users:\nUserInfo{0:Owner:13} running\nUserInfo{10:Secondary:10}\n' ;;
  "shell pm list packages -U -u -i --user 0") printf 'package:org.main installer=null uid:10123\n' ;;
  "shell pm list packages --user 0") printf 'package:org.main\n' ;;
  "shell pm list packages --user 0 -d"|"shell pm list packages --user 0 -s"|"shell pm list packages --user 0 -3") ;;
  "shell pm path --user 0 "*) printf 'package:/data/app/org.main/base.apk\n' ;;
  *"--user 10") printf 'SecurityException: Shell does not have permission to access user 10\n' >&2; exit 1 ;;
  *) printf 'unexpected command: %s\n' "$*" >&2; exit 2 ;;
esac
`
	if err := os.WriteFile(fakeADB, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	oldClient := adb.Client
	adb.Client = &adb.ADB{ExePath: fakeADB}
	t.Cleanup(func() { adb.Client = oldClient })
	writer, err := acquisition.NewStreamingZipWriter("per-user-partial", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	acq := &acquisition.Acquisition{ZipWriter: writer}
	err = NewPackages().Run(acq, &Options{Fast: true, NonInteractive: true, Download: apkNone})
	if !errors.Is(err, ErrPartialCollection) {
		t.Fatalf("Run() = %v, want partial collection", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(writer.GetOutputPath())
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	entries := map[string][]byte{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name], err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	var packages []adb.Package
	if err := json.Unmarshal(entries["packages.json"], &packages); err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].UserID != 0 || packages[0].Name != "org.main" {
		t.Fatalf("lost accessible user's package: %+v", packages)
	}
	var report adb.PackageUsersReport
	if err := json.Unmarshal(entries["package_users.json"], &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Users) != 2 || report.Users[0].Status != "collected" || report.Users[1].Status != "failed" || report.Users[1].Error == "" {
		t.Fatalf("missing denied user's outcome: %+v", report)
	}
}

func TestPackageDownloadRecordsUniqueArchiveEntryForEachUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX fake ADB executable")
	}
	fakeADB := filepath.Join(t.TempDir(), "adb")
	if err := os.WriteFile(fakeADB, []byte("#!/bin/sh\nprintf 'test transfer bytes'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writer, err := acquisition.NewStreamingZipWriter("shared-apk", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	acq := &acquisition.Acquisition{ZipWriter: writer, StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "", 1)}
	used := map[string]struct{}{}
	files := []adb.PackageFile{{Path: "/data/app/org.shared/base.apk"}, {Path: "/data/app/org.shared/base.apk"}}
	for i := range files {
		if err := NewPackages().processAPKStreaming("org.shared", &files[i], apkKeepAll, acq, used); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if files[0].LocalName == "" || files[1].LocalName == "" || files[0].LocalName == files[1].LocalName {
		t.Fatalf("ambiguous APK provenance: %+v", files)
	}
	archive, err := zip.OpenReader(writer.GetOutputPath())
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range files {
		entry, err := archive.Open(file.LocalName)
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(entry)
		entry.Close()
		if err != nil || string(content) != "test transfer bytes" {
			t.Fatalf("archive entry %s = %q, error = %v", file.LocalName, content, err)
		}
	}
}
