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

func TestInitScripts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}
	for _, tc := range []struct {
		name    string
		root    string
		listing string
		partial bool
		want    map[string]string
	}{
		{name: "no root", root: "printf 2000"},
		{name: "su denied", root: "exit 1"},
		{name: "missing directory", root: "printf 0", listing: "exit 0"},
		{name: "recursive files", root: "printf 0", listing: `printf '/system/etc/init.d/00 start\0/system/etc/init.d/nested/.hidden\0'`, want: map[string]string{"init_scripts/00 start": "first", "init_scripts/nested/.hidden": "second"}},
		{name: "unsafe path", root: "printf 0", listing: `printf '/system/etc/init.d/../../secret\0'`, partial: true},
		{name: "listing failure", root: "printf 0", listing: "exit 1", partial: true},
		{name: "failed pull continues", root: "printf 0", listing: `printf '/system/etc/init.d/broken\0/system/etc/init.d/00 start\0'`, partial: true, want: map[string]string{"init_scripts/00 start": "first"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fakeADB := filepath.Join(dir, "adb")
			calls := filepath.Join(dir, "calls")
			script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + calls + "'\ncase \"$*\" in\n" +
				"*\"id -u\"*) " + tc.root + " ;;\n" +
				"*\"find /system/etc/init.d/\"*) " + tc.listing + " ;;\n" +
				`*"cat --"*"00 start"*) printf first ;;
*"cat --"*"nested/.hidden"*) printf second ;;
*"cat --"*"broken"*) printf incomplete; exit 1 ;;
*) exit 1 ;;
esac
`
			// A no-root case must never get past the root probe.
			if tc.listing == "" {
				script = strings.Replace(script, "*)  ;;", "*) exit 99 ;;", 1)
			}
			if err := os.WriteFile(fakeADB, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			oldClient := adb.Client
			adb.Client = &adb.ADB{ExePath: fakeADB}
			t.Cleanup(func() { adb.Client = oldClient })
			writer, err := acquisition.NewStreamingZipWriter("init-scripts", dir)
			if err != nil {
				t.Fatal(err)
			}
			acq := &acquisition.Acquisition{ZipWriter: writer, StreamingMode: true, StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "", 1)}
			err = NewInitScripts().Run(acq, &Options{})
			if tc.partial {
				if !errors.Is(err, ErrPartialCollection) {
					t.Fatalf("expected partial collection, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.OpenReader(writer.GetOutputPath())
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			if len(archive.File) != len(tc.want) {
				t.Fatalf("archive contains %d files, want %d", len(archive.File), len(tc.want))
			}
			for _, file := range archive.File {
				reader, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				want, ok := tc.want[file.Name]
				if !ok || string(content) != want {
					t.Fatalf("unexpected entry %q: %q", file.Name, content)
				}
			}
			if tc.listing == "" {
				data, err := os.ReadFile(calls)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(string(data), "\n") != 1 || !strings.Contains(string(data), "id -u") {
					t.Fatalf("commands ran without root: %s", data)
				}
			}
		})
	}
}

func TestInitScriptsWithoutADB(t *testing.T) {
	oldClient := adb.Client
	adb.Client = nil
	defer func() { adb.Client = oldClient }()
	if err := NewInitScripts().Run(nil, nil); err != nil {
		t.Fatal(err)
	}
}
