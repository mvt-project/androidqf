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
	"time"

	"filippo.io/age"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
)

// The fixture permits only property reads, access probes, find and cat. It
// records all commands, including the serial and su invocations.
const modemFakeADB = `#!/bin/sh
printf '%s\n' "$*" >> "$MODEM_TEST_DIR/calls"
case "$*" in
  *"getprop"*) printf '[ro.product.manufacturer]: [Example]\n[ro.product.model]: [Unknown modem]\n[gsm.version.baseband]: [test-baseband]\n' ;;
  *"id -u"*) printf '%s' "${MODEM_TEST_ROOT:-2000}" ;;
  *"find . -type f -print0"*)
    case "$*" in
      *"/sdcard/diag_logs"*)
        case "$*" in
          *"su -c"*) cat "$MODEM_TEST_DIR/root-listing"; exit "${MODEM_TEST_ROOT_LIST_EXIT:-0}" ;;
          *) cat "$MODEM_TEST_DIR/listing"; exit "${MODEM_TEST_LIST_EXIT:-0}" ;;
        esac ;;
      *"/data/vendor/radio/logs"*) printf './protected.sdm\0' ;;
      *) exit 98 ;;
    esac ;;
  *"printf readable"*)
    case "$*" in
      *"/data/vendor/radio/logs"*)
        if [ "${MODEM_TEST_PRIVATE:-0}" = 1 ]; then
          case "$*" in
            *"su -c"*) printf readable ;;
            *) printf inaccessible ;;
          esac
        else printf not_found; fi ;;
      *"/sdcard/diag_logs"*)
        case "$*" in
          *"root-only.dlf"*)
            case "$*" in
              *"su -c"*) printf readable ;;
              *) printf inaccessible ;;
            esac ;;
          *) printf readable ;;
        esac ;;
      *) printf not_found ;;
    esac ;;
  *"exec-out"*)
    case "$*" in
      *"blocking.qmdl"*) touch "$MODEM_TEST_DIR/pulling"; exec sleep 30 ;;
    esac
    cat "$MODEM_TEST_DIR/payload"
    case "$*" in *"broken.qmdl"*) exit 1 ;; esac ;;
  *) exit 99 ;;
esac
`

func modemTestDevice(t *testing.T, listing string, encrypted bool) (*acquisition.Acquisition, *age.X25519Identity) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake ADB shell fixture requires Unix")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("MODEM_TEST_DIR", dir)
	for name, data := range map[string]string{
		"adb": modemFakeADB, "listing": listing, "root-listing": listing,
		"payload": "raw\x00modem\xff\r\nlog", "calls": "",
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
	adb.Client = &adb.ADB{ExePath: fakeADB, Serial: "modem-test-device"}
	t.Cleanup(func() { adb.Client = old })
	writer, err := acquisition.NewStreamingZipWriter("modem-test", dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close() })
	return &acquisition.Acquisition{
		ZipWriter: writer, StreamingMode: true,
		StreamingPuller: acquisition.NewStreamingPuller(fakeADB, "modem-test-device", 1),
	}, identity
}

func readModemTestArchive(t *testing.T, acq *acquisition.Acquisition, identity *age.X25519Identity) (modemDiagnosticsManifest, map[string][]byte) {
	t.Helper()
	if err := acq.ZipWriter.CreateHashList(); err != nil {
		t.Fatal(err)
	}
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
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name], err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	var manifest modemDiagnosticsManifest
	if err := json.Unmarshal(files["modem_diagnostics/manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest, files
}

func modemTestCalls(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("MODEM_TEST_DIR"), "calls"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestModemDiagnosticsOptOutDoesNotAccessDevice(t *testing.T) {
	old := adb.Client
	adb.Client = nil
	t.Cleanup(func() { adb.Client = old })
	for _, nonInteractive := range []bool{false, true} {
		if err := NewModemDiagnostics().Run(nil, &Options{NonInteractive: nonInteractive, ModemDiagnostics: skipModemDiagnostics}); err != nil {
			t.Fatal(err)
		}
	}
	err := NewModemDiagnostics().Run(nil, &Options{NonInteractive: true})
	if err == nil || !strings.Contains(err.Error(), "-modem-diagnostics") {
		t.Fatalf("missing choice: got %v", err)
	}
}

func TestModemDiagnosticsCollectsRootlessLogs(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "encrypted"}[encrypted], func(t *testing.T) {
			acq, identity := modemTestDevice(t, "./trace.qmdl\x00./nested/call's\ntrace.SDM.GZ\x00./trace.qmdl\x00./notes:unrelated.txt\x00./unsupported.qdss\x00./export.lpd\x00", encrypted)
			// Fast mode and an unknown manufacturer must not suppress opted-in logs.
			if err := NewModemDiagnostics().Run(acq, &Options{NonInteractive: true, Fast: true, ModemDiagnostics: acquireModemDiagnostics}); err != nil {
				t.Fatal(err)
			}
			manifest, files := readModemTestArchive(t, acq, identity)
			if manifest.Status != "collected" || manifest.RootAccess != "not_checked" || len(manifest.Files) != 3 || len(manifest.Sources) != len(modemDiagnosticRoots) {
				t.Fatalf("unexpected manifest: %+v", manifest)
			}
			if manifest.Properties["ro.product.model"] != "Unknown modem" || manifest.Properties["gsm.version.baseband"] != "test-baseband" {
				t.Fatalf("lost device properties: %v", manifest.Properties)
			}
			for _, name := range []string{"trace.qmdl", "nested/call's\ntrace.SDM.GZ", "export.lpd"} {
				if got := string(files["modem_diagnostics/files/sdcard/diag_logs/"+name]); got != "raw\x00modem\xff\r\nlog" {
					t.Fatalf("%s: did not preserve binary log: %q", name, got)
				}
			}
			if !bytes.Contains(files["hashes.csv"], []byte("modem_diagnostics/files/sdcard/diag_logs/trace.qmdl")) {
				t.Fatal("modem logs missing from integrity hash list")
			}
			if len(files) != 5 { // Three logs, manifest and hashes; not text or QDSS files.
				t.Fatalf("unexpected archive entries: %v", files)
			}
			calls := modemTestCalls(t)
			if strings.Contains(calls, "su -c") || strings.Contains(calls, "id -u") {
				t.Fatalf("root was requested unnecessarily: %s", calls)
			}
			if got := strings.Count(calls, "exec-out"); got != 3 {
				t.Fatalf("pulled %d files, want 3", got)
			}
			for _, forbidden := range []string{"setprop", "chmod", "setenforce", "diag_mdlog", "modemlogcat", " root", " forward"} {
				if strings.Contains(calls, forbidden) {
					t.Fatalf("unexpected device mutation: %s", calls)
				}
			}
			if !strings.Contains(calls, "-s modem-test-device exec-out") {
				t.Fatal("pull did not select the requested device")
			}
		})
	}
}

func TestModemDiagnosticsRootFallback(t *testing.T) {
	for _, rootAvailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "unavailable", true: "available"}[rootAvailable], func(t *testing.T) {
			acq, identity := modemTestDevice(t, "", false)
			t.Setenv("MODEM_TEST_PRIVATE", "1")
			if rootAvailable {
				t.Setenv("MODEM_TEST_ROOT", "0")
			}
			if err := NewModemDiagnostics().Run(acq, &Options{ModemDiagnostics: acquireModemDiagnostics}); err != nil {
				t.Fatal(err)
			}
			manifest, _ := readModemTestArchive(t, acq, identity)
			if rootAvailable {
				if manifest.Status != "collected" || manifest.RootAccess != "available" || len(manifest.Files) != 1 || manifest.Files[0].Method != "su" {
					t.Fatalf("root fallback failed: %+v", manifest)
				}
			} else if manifest.Status != "access_limited" || manifest.RootAccess != "unavailable" || len(manifest.Files) != 0 {
				t.Fatalf("inaccessible logs treated as absent: %+v", manifest)
			}
			if got := strings.Count(modemTestCalls(t), "id -u"); got != 1 {
				t.Fatalf("root probed %d times, want once", got)
			}
		})
	}
}

func TestModemDiagnosticsRootOnlyFileInReadableDirectory(t *testing.T) {
	acq, identity := modemTestDevice(t, "./root-only.dlf\x00./other.qmdl\x00", false)
	t.Setenv("MODEM_TEST_ROOT", "0")
	if err := NewModemDiagnostics().Run(acq, &Options{ModemDiagnostics: acquireModemDiagnostics}); err != nil {
		t.Fatal(err)
	}
	manifest, _ := readModemTestArchive(t, acq, identity)
	if manifest.Status != "collected" || len(manifest.Files) != 2 || manifest.Files[0].Method != "adb" || manifest.Files[1].Method != "su" {
		t.Fatalf("per-file root access not respected: %+v", manifest)
	}
}

func TestModemDiagnosticsFailedTransferIsNotArchived(t *testing.T) {
	acq, identity := modemTestDevice(t, "./broken.qmdl\x00./complete.qmdl\x00", false)
	err := NewModemDiagnostics().Run(acq, &Options{ModemDiagnostics: acquireModemDiagnostics})
	if !errors.Is(err, ErrPartialCollection) {
		t.Fatalf("got %v, want partial collection", err)
	}
	manifest, files := readModemTestArchive(t, acq, identity)
	if manifest.Status != "partial" || len(manifest.Files) != 2 || manifest.Files[0].Status != "failed" || manifest.Files[1].Status != "collected" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if _, exists := files["modem_diagnostics/files/sdcard/diag_logs/broken.qmdl"]; exists {
		t.Fatal("failed transfer committed an archive entry")
	}
	if strings.Contains(modemTestCalls(t), "su -c") {
		t.Fatal("failed staging/archive write retried as root")
	}
}

func TestModemDiagnosticsPartialListing(t *testing.T) {
	for _, rootAvailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial", true: "recovered"}[rootAvailable], func(t *testing.T) {
			acq, identity := modemTestDevice(t, "./complete.qmdl\x00", false)
			t.Setenv("MODEM_TEST_LIST_EXIT", "1")
			if rootAvailable {
				t.Setenv("MODEM_TEST_ROOT", "0")
				if err := os.WriteFile(filepath.Join(os.Getenv("MODEM_TEST_DIR"), "root-listing"), []byte("./complete.qmdl\x00./root-only.dlf\x00"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := NewModemDiagnostics().Run(acq, &Options{ModemDiagnostics: acquireModemDiagnostics})
			if rootAvailable && err != nil || !rootAvailable && !errors.Is(err, ErrPartialCollection) {
				t.Fatalf("unexpected error: %v", err)
			}
			manifest, files := readModemTestArchive(t, acq, identity)
			if _, exists := files["modem_diagnostics/files/sdcard/diag_logs/complete.qmdl"]; !exists {
				t.Fatal("partial listing lost a readable file")
			}
			if rootAvailable {
				if manifest.Status != "collected" || len(manifest.Files) != 2 {
					t.Fatalf("root retry failed to recover listing: %+v", manifest)
				}
			} else if manifest.Status != "partial" || manifest.Sources[0].Status != "partial" {
				t.Fatalf("listing failure not reported: %+v", manifest)
			}
		})
	}
}

func TestModemDiagnosticsRejectsTruncatedListing(t *testing.T) {
	acq, identity := modemTestDevice(t, "./complete.qmdl\x00./truncated.qmdl", false)
	err := NewModemDiagnostics().Run(acq, &Options{ModemDiagnostics: acquireModemDiagnostics})
	if !errors.Is(err, ErrPartialCollection) {
		t.Fatalf("got %v, want partial collection", err)
	}
	manifest, _ := readModemTestArchive(t, acq, identity)
	if len(manifest.Files) != 1 || !strings.HasSuffix(manifest.Files[0].DevicePath, "/complete.qmdl") {
		t.Fatalf("truncated filename was used: %+v", manifest)
	}
}

func TestModemDiagnosticsEmptyAndCanceled(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "canceled"}[canceled], func(t *testing.T) {
			acq, identity := modemTestDevice(t, "", false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
			}
			err := NewModemDiagnostics().Run(acq, &Options{ModemDiagnostics: acquireModemDiagnostics, Context: ctx})
			if canceled && !errors.Is(err, ErrAcquisitionInterrupted) || !canceled && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			manifest, _ := readModemTestArchive(t, acq, identity)
			wantStatus := "no_logs"
			if canceled {
				wantStatus = "interrupted"
				if modemTestCalls(t) != "" {
					t.Fatal("accessed device after cancellation")
				}
			}
			if manifest.Status != wantStatus || len(manifest.Files) != 0 {
				t.Fatalf("unexpected manifest: %+v", manifest)
			}
		})
	}
}

func TestModemDiagnosticsCancelsActiveTransfer(t *testing.T) {
	acq, identity := modemTestDevice(t, "./a.qmdl\x00./blocking.qmdl\x00", false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	acq.StreamingPuller.SetContext(ctx)
	done := make(chan error, 1)
	go func() {
		done <- NewModemDiagnostics().Run(acq, &Options{ModemDiagnostics: acquireModemDiagnostics, Context: ctx})
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
waitForPull:
	for {
		select {
		case <-deadline.C:
			t.Fatal("transfer did not start")
		case <-poll.C:
			if _, err := os.Stat(filepath.Join(os.Getenv("MODEM_TEST_DIR"), "pulling")); err == nil {
				break waitForPull
			}
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrAcquisitionInterrupted) {
			t.Fatalf("got %v, want interrupted acquisition", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transfer did not stop after cancellation")
	}
	manifest, files := readModemTestArchive(t, acq, identity)
	if manifest.Status != "interrupted" || len(manifest.Files) != 2 || manifest.Files[0].Status != "collected" || manifest.Files[1].Status != "failed" {
		t.Fatalf("unexpected partial manifest: %+v", manifest)
	}
	if _, exists := files["modem_diagnostics/files/sdcard/diag_logs/blocking.qmdl"]; exists {
		t.Fatal("canceled transfer committed an archive entry")
	}
}

func TestModemDiagnosticPaths(t *testing.T) {
	for _, name := range []string{"../evil.qmdl", "./../evil.qmdl", "./sub/../evil.qmdl", "./sub//evil.qmdl", "/etc/evil.qmdl", "./C:evil.qmdl", "./back\\evil.qmdl", "./null\x00.qmdl", "./"} {
		if _, _, err := modemDiagnosticPaths("/sdcard/log", name); err == nil {
			t.Errorf("accepted unsafe path %q", name)
		}
	}
	device, archive, err := modemDiagnosticPaths("/sdcard/log", "./call's\ntrace.sdm")
	if err != nil || device != "/sdcard/log/call's\ntrace.sdm" || archive != "modem_diagnostics/files/sdcard/log/call's\ntrace.sdm" {
		t.Fatalf("safe path changed: %q, %q, %v", device, archive, err)
	}
}

func TestModemDiagnosticFormats(t *testing.T) {
	for filename, want := range map[string]string{
		"a.qmdl": "qmdl", "a.QMDL2.GZ": "qmdl2", "a.dlf": "dlf", "a.sdm.zip": "sdm",
		"a.sdm.lz4": "sdm", "a.lpd.zst": "lpd", "a.log": "", "a.qdss": "", "a.qmdl.txt": "", "a.zip": "",
	} {
		if got := modemDiagnosticFormat(filename); got != want {
			t.Errorf("format(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestModemDiagnosticsRunsBeforeBugreport(t *testing.T) {
	seen := false
	for _, mod := range List() {
		if mod.Name() == "modem_diagnostics" {
			seen = true
		}
		if mod.Name() == "bugreport" && !seen {
			t.Fatal("modem diagnostics must run before bugreport")
		}
	}
	if !seen {
		t.Fatal("modem diagnostics is not registered")
	}
}

func TestModemDiagnosticCommandsPreserveNamesAndSkipSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires Unix")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	fakeADB := filepath.Join(dir, "adb")
	// Execute the actual generated device commands in an isolated local tree,
	// not just pattern-match them, to verify quoting and find semantics.
	if err := os.WriteFile(fakeADB, []byte("#!/bin/sh\n[ \"$1\" = shell ] || exit 99\nshift\nexec sh -c \"$*\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(filepath.Join(logs, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	name := "nested/trace'$(touch UNEXPECTED)\n.sdm"
	if err := os.WriteFile(filepath.Join(logs, name), []byte("log"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside.qmdl")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(logs, "link.qmdl")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(logs, "linked-directory")); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "sdcard-link")
	if err := os.Symlink(logs, root); err != nil {
		t.Fatal(err)
	}
	client := &adb.ADB{ExePath: fakeADB}
	out, err := listModemLogs(client, root, false)
	if err != nil || out != "./"+name+"\x00" {
		t.Fatalf("listing = %q, %v", out, err)
	}
	for _, tt := range []struct {
		path      string
		directory bool
		want      string
	}{
		{root, true, "readable"},
		{filepath.Join(root, name), false, "readable"},
		{filepath.Join(root, "link.qmdl"), false, "symlink"},
		{filepath.Join(root, "nested"), false, "not_regular"},
		{filepath.Join(root, "missing"), true, "not_found"},
	} {
		got, err := probeModemPath(client, tt.path, tt.directory, false)
		if err != nil || got != tt.want {
			t.Errorf("probe(%q) = %q, %v, want %q", tt.path, got, err, tt.want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "UNEXPECTED")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("filename executed as shell syntax: %v", err)
	}
}
