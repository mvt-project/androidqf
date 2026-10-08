package modules

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
)

const testApexInventory = `<?xml version="1.0"?>
<apex-info-list>
  <apex-info moduleName="com.example.test" modulePath="/data/apex/active/test.apex" preinstalledModulePath="/system/apex/test.apex" versionCode="123" isFactory="false" isActive="true" futureAttribute="preserved"/>
  <apex-info moduleName="com.example.test" modulePath="/system/apex/test.apex" preinstalledModulePath="/system/apex/test.apex" versionCode="100" isFactory="true" isActive="false"/>
  <apex-info moduleName="com.example.compressed" modulePath="/product/apex/test.capex" isFactory="true" isActive="true"/>
</apex-info-list>
`

// The fake device returns opaque containers: no signature or key assessment is
// needed to acquire them. Failures can emit bytes before exiting unsuccessfully.
const apexFakeADB = `#!/bin/sh
printf '%s\n' "$*" >> "$APEX_TEST_DIR/calls"
case "$*" in
  *"getprop ro.build.version.sdk"*) printf '%s' "${APEX_TEST_SDK:-34}" ;;
  *"exec-out cat /apex/apex-info-list.xml"*) cat "$APEX_TEST_DIR/inventory"; exit "${APEX_TEST_INVENTORY_EXIT:-0}" ;;
  *"pm list packages --apex-only -f"*) printf 'package:/data/apex/active/test.apex=com.example.test\n' ;;
  *"for d in /system/apex"*) printf '/system/apex/test.apex\0/product/apex/test.capex\0' ;;
  *"id -u"*) printf '%s' "${APEX_TEST_ROOT:-2000}" ;;
  *"printf directory"*)
    case "$*" in
      *"/system/apex/flat"*) printf directory ;;
      *"/data/apex/active/test.apex"*)
        case "$*" in
          *"su -c"*) printf file ;;
          *) printf '%s' "${APEX_TEST_ACTIVE_KIND:-file}" ;;
        esac ;;
      *) printf file ;;
    esac ;;
  *"exec-out"*)
    case "$*" in
      *"/system/apex/test.apex"*) cat "$APEX_TEST_DIR/factory" ;;
      *"/product/apex/test.capex"*) cat "$APEX_TEST_DIR/compressed" ;;
      *"/data/apex/active/test.apex"*) cat "$APEX_TEST_DIR/active"; exit "${APEX_TEST_PULL_EXIT:-0}" ;;
      *) exit 98 ;;
    esac ;;
  *) exit 99 ;;
esac
`

func apexTestDevice(t *testing.T, inventory string, encrypted bool) (*acquisition.Acquisition, *age.X25519Identity) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake ADB shell fixture requires Unix")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("APEX_TEST_DIR", dir)
	for name, data := range map[string]string{
		"adb": apexFakeADB, "inventory": inventory,
		"factory": "factory\x00APEX\xff", "active": "updated\x00APEX\xfe", "compressed": "compressed\x00CAPEX\xfd",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var identity *age.X25519Identity
	if encrypted {
		var err error
		identity, err = age.GenerateX25519Identity()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "key.txt"), []byte(identity.Recipient().String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fakeADB := filepath.Join(dir, "adb")
	old := adb.Client
	adb.Client = &adb.ADB{ExePath: fakeADB, Serial: "apex-test-device"}
	t.Cleanup(func() { adb.Client = old })
	writer, err := acquisition.NewStreamingZipWriter("apex-test", dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close() })
	acq := &acquisition.Acquisition{
		ZipWriter: writer, StreamingMode: true,
		StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "apex-test-device", 1),
	}
	return acq, identity
}

func readApexTestArchive(t *testing.T, acq *acquisition.Acquisition, identity *age.X25519Identity) (apexManifest, map[string][]byte) {
	t.Helper()
	if err := acq.ZipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(acq.ZipWriter.GetOutputPath())
	if err != nil {
		t.Fatal(err)
	}
	if identity != nil {
		reader, err := age.Decrypt(bytes.NewReader(data), identity)
		if err != nil {
			t.Fatal(err)
		}
		data, err = io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for _, file := range archive.File {
		if _, exists := files[file.Name]; exists {
			t.Fatalf("duplicate archive entry %q", file.Name)
		}
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name], err = io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	var manifest apexManifest
	if err := json.Unmarshal(files["apex/manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest, files
}

func TestApexPreservesContainersAndInventory(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "encrypted"}[encrypted], func(t *testing.T) {
			acq, identity := apexTestDevice(t, testApexInventory, encrypted)
			// APK download choices must not suppress APEX evidence.
			err := NewApex().Run(acq, &Options{NonInteractive: true, Download: apkNone, RemoveTrusted: apkRemoveTrusted})
			if err != nil {
				t.Fatal(err)
			}
			manifest, files := readApexTestArchive(t, acq, identity)
			if manifest.Status != "completed" || len(manifest.Modules) != 3 || len(manifest.Files) != 3 {
				t.Fatalf("unexpected manifest: %+v", manifest)
			}
			if manifest.Modules[0].IsActive == nil || !*manifest.Modules[0].IsActive || *manifest.Modules[0].IsFactory || manifest.Modules[0].VersionCode != "123" {
				t.Fatalf("lost inventory relationships: %+v", manifest.Modules[0])
			}
			for name, want := range map[string]string{
				"apex/apex-info-list.xml":               testApexInventory,
				"apex/files/system/apex/test.apex":      "factory\x00APEX\xff",
				"apex/files/data/apex/active/test.apex": "updated\x00APEX\xfe",
				"apex/files/product/apex/test.capex":    "compressed\x00CAPEX\xfd",
			} {
				if string(files[name]) != want {
					t.Fatalf("%s was not preserved byte-for-byte", name)
				}
			}
			calls, err := os.ReadFile("calls")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(calls), "su -c") || strings.Contains(string(calls), "pm list") {
				t.Fatalf("unexpected fallback commands: %s", calls)
			}
			for _, entry := range manifest.Files {
				if entry.Status != "collected" || entry.Method != "adb" || files[entry.ArchivePath] == nil {
					t.Fatalf("invalid file provenance: %+v", entry)
				}
			}
		})
	}
}

func TestApexPartialTransfersAndRoot(t *testing.T) {
	for _, tc := range []struct {
		name, kind, root, pullExit string
		partial                    bool
	}{
		{name: "unrooted denial", kind: "inaccessible", root: "2000", partial: true},
		{name: "existing root", kind: "inaccessible", root: "0"},
		{name: "truncated transfer", kind: "file", pullExit: "1", partial: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acq, identity := apexTestDevice(t, testApexInventory, false)
			t.Setenv("APEX_TEST_ACTIVE_KIND", tc.kind)
			t.Setenv("APEX_TEST_ROOT", tc.root)
			t.Setenv("APEX_TEST_PULL_EXIT", tc.pullExit)
			err := NewApex().Run(acq, &Options{})
			if errors.Is(err, ErrPartialCollection) != tc.partial || (!tc.partial && err != nil) {
				t.Fatalf("unexpected error: %v", err)
			}
			manifest, files := readApexTestArchive(t, acq, identity)
			_, active := files["apex/files/data/apex/active/test.apex"]
			if active == tc.partial || files["apex/files/system/apex/test.apex"] == nil {
				t.Fatal("failed transfer was archived or later evidence was lost")
			}
			entry := manifest.Files[0]
			if tc.partial && (manifest.Status != "partial" || entry.Error == "" || entry.ArchivePath != "") {
				t.Fatalf("failure provenance missing: %+v", manifest)
			}
			if tc.root == "0" && entry.Method != "su" {
				t.Fatalf("root acquisition not recorded: %+v", entry)
			}
			calls, _ := os.ReadFile("calls")
			if tc.root == "2000" && strings.Contains(string(calls), "exec-out su") {
				t.Fatal("root transfer attempted without root")
			}
		})
	}
}

func TestApexFallbackPreservesInvalidInventory(t *testing.T) {
	for _, raw := range []string{"", "<broken>", "<unexpected/>"} {
		t.Run(raw, func(t *testing.T) {
			acq, identity := apexTestDevice(t, raw, false)
			if err := NewApex().Run(acq, &Options{}); !errors.Is(err, ErrPartialCollection) {
				t.Fatalf("expected incomplete inventory: %v", err)
			}
			manifest, files := readApexTestArchive(t, acq, identity)
			if len(manifest.Files) != 3 || manifest.Inventory != "package_manager_and_factory_directories" || files["apex/packages.txt"] == nil {
				t.Fatalf("fallback lost evidence: %+v", manifest)
			}
			if raw != "" && string(files["apex/apex-info-list.xml"]) != raw {
				t.Fatal("invalid inventory was discarded")
			}
		})
	}
}

func TestApexDirectoriesAndOldAndroid(t *testing.T) {
	t.Run("flattened", func(t *testing.T) {
		acq, identity := apexTestDevice(t, `<apex-info-list><apex-info moduleName="flat" modulePath="/system/apex/flat"/></apex-info-list>`, false)
		if err := NewApex().Run(acq, &Options{}); err != nil {
			t.Fatal(err)
		}
		manifest, files := readApexTestArchive(t, acq, identity)
		if manifest.Files[0].Status != "directory" || len(files) != 2 {
			t.Fatalf("flattened APEX was treated as a signed container: %+v", manifest)
		}
	})
	t.Run("pre Android 10", func(t *testing.T) {
		acq, identity := apexTestDevice(t, "", false)
		t.Setenv("APEX_TEST_SDK", "28")
		if err := NewApex().Run(acq, &Options{}); err != nil {
			t.Fatal(err)
		}
		manifest, files := readApexTestArchive(t, acq, identity)
		calls, _ := os.ReadFile("calls")
		if manifest.Status != "not_supported" || len(files) != 1 || strings.Count(string(calls), "\n") != 1 {
			t.Fatalf("unsupported device was not skipped: %+v; %s", manifest, calls)
		}
	})
}

func TestApexRejectsUnsafeInventoryPaths(t *testing.T) {
	for _, devicePath := range []string{"/data/secret.apex", "/system/apex/../../secret.apex", "relative.apex", "/system/apex\\secret.apex", "/system/apex/", "/data/apex\x00/foo.apex"} {
		if _, err := apexArchivePath(devicePath); err == nil {
			t.Fatalf("unsafe path accepted: %q", devicePath)
		}
	}
	acq, identity := apexTestDevice(t, `<apex-info-list><apex-info moduleName="bad" modulePath="/data/secret.apex"/></apex-info-list>`, false)
	if err := NewApex().Run(acq, &Options{}); !errors.Is(err, ErrPartialCollection) {
		t.Fatalf("unsafe inventory did not produce partial result: %v", err)
	}
	manifest, files := readApexTestArchive(t, acq, identity)
	calls, _ := os.ReadFile("calls")
	if len(files) != 2 || manifest.Files[0].Error == "" || strings.Contains(string(calls), "/data/secret.apex") {
		t.Fatal("unsafe path was accessed or error evidence was lost")
	}
}

func TestApexCancellationKeepsManifest(t *testing.T) {
	acq, identity := apexTestDevice(t, testApexInventory, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewApex().Run(acq, &Options{Context: ctx})
	if !errors.Is(err, ErrAcquisitionInterrupted) {
		t.Fatalf("cancellation was not reported: %v", err)
	}
	manifest, _ := readApexTestArchive(t, acq, identity)
	if manifest.Status != "partial" || len(manifest.Files) != 0 {
		t.Fatalf("unexpected interrupted manifest: %+v", manifest)
	}
}
