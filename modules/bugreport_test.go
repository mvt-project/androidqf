package modules

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
)

func TestBugreportPreservesExistingFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}
	for _, scenario := range []string{"reports", "missing", "empty", "listing-failed", "pull-failed", "generation-failed", "unsafe-path"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("BUGREPORT_TEST_DIR", dir)
			t.Setenv("BUGREPORT_TEST_SCENARIO", scenario)
			fakeADB := filepath.Join(dir, "adb")
			// Execute the real listing command against a local symlink fixture.
			// Generating a new report removes the old source, enforcing ordering.
			script := `#!/bin/sh
case "$1" in
shell)
  shift
  if [ "$1" = bugreportz ]; then
    [ "$BUGREPORT_TEST_SCENARIO" = generation-failed ] && exit 1
    rm -f "$BUGREPORT_TEST_DIR/source/old report.zip"
    printf 'OK:/fresh.zip\n'
  elif [ "$1" = rm ]; then
    exit 0
  else
    case "$BUGREPORT_TEST_SCENARIO" in
      listing-failed) exit 1 ;;
      unsafe-path) printf './../../escape.zip\000'; exit 0 ;;
    esac
    command=$(printf '%s' "$*" | sed "s|/bugreports|$BUGREPORT_TEST_DIR/bugreports|g")
    sh -c "$command"
  fi ;;
exec-out)
  case "$*" in
    *fresh.zip*) printf 'fresh report' ;;
    *failed.zip*) printf 'truncated'; exit 1 ;;
    *)
      shift
      command=$(printf '%s' "$*" | sed "s|/bugreports|$BUGREPORT_TEST_DIR/bugreports|g")
      sh -c "$command" ;;
  esac ;;
*) exit 1 ;;
esac
`
			if err := os.WriteFile(fakeADB, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"bugreport.zip": "fresh report"}
			if scenario != "missing" {
				if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "source"), filepath.Join(dir, "bugreports")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "reports" || scenario == "pull-failed" || scenario == "generation-failed" {
				for _, name := range []string{"old report.zip", "screenshot\n'1.png", "dumpstate_log.txt"} {
					if err := os.WriteFile(filepath.Join(dir, "source", name), []byte(name), 0600); err != nil {
						t.Fatal(err)
					}
					want["bugreports/"+name] = name
				}
				if err := os.Symlink(fakeADB, filepath.Join(dir, "source", "outside")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "pull-failed" {
				if err := os.WriteFile(filepath.Join(dir, "source", "failed.zip"), []byte("report"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			oldClient := adb.Client
			adb.Client = &adb.ADB{ExePath: fakeADB}
			t.Cleanup(func() { adb.Client = oldClient })
			writer, err := acquisition.NewStreamingZipWriter("bugreport-test", dir)
			if err != nil {
				t.Fatal(err)
			}
			acq := &acquisition.Acquisition{
				ZipWriter: writer, StreamingMode: true,
				StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "", 1),
			}
			err = NewBugreport().Run(acq, &Options{})
			switch scenario {
			case "listing-failed", "pull-failed", "unsafe-path":
				if !errors.Is(err, ErrPartialCollection) {
					t.Fatalf("Run() = %v, want partial collection", err)
				}
			case "generation-failed":
				delete(want, "bugreport.zip")
				if err == nil || !strings.Contains(err.Error(), "failed to stream bugreport") {
					t.Fatalf("Run() = %v, want generation failure", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.OpenReader(writer.GetOutputPath())
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			if len(archive.File) != len(want) {
				t.Fatalf("archive has %d entries, want %d", len(archive.File), len(want))
			}
			for _, file := range archive.File {
				r, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(r)
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				if expected, ok := want[file.Name]; !ok || string(content) != expected {
					t.Fatalf("unexpected archive entry %q: %q", file.Name, content)
				}
			}
		})
	}
}
