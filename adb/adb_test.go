package adb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("ANDROIDQF_FAKE_ADB") == "1" {
		fakeADB()
		return
	}
	os.Exit(m.Run())
}

func fakeADB() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}

	switch os.Args[1] {
	case "devices":
		fmt.Println("List of devices attached")
		for _, device := range strings.Split(os.Getenv("ANDROIDQF_FAKE_ADB_DEVICES"), ",") {
			device = strings.TrimSpace(device)
			if device != "" {
				if len(os.Args) > 2 && os.Args[2] == "-l" {
					fmt.Printf("%s         device product:fake model:%s_Model device:%s transport_id:1\n", device, device, device)
				} else {
					fmt.Printf("%s\tdevice\n", device)
				}
			}
		}
	case "pubkey":
		fmt.Println(os.Getenv("ANDROIDQF_FAKE_ADB_PUBLIC_KEY"))
	case "wait":
		_ = os.WriteFile(os.Getenv("ANDROIDQF_FAKE_ADB_MARKER"), []byte("started"), 0o600)
		select {}
	case "shell":
		if os.Getenv("ANDROIDQF_FAKE_ADB_REQUIRE_TYPE_FILE") == "1" && !strings.Contains(strings.Join(os.Args[2:], " "), "-type f") {
			os.Exit(2)
		}
		fmt.Print(os.Getenv("ANDROIDQF_FAKE_ADB_SHELL_OUTPUT"))
		if os.Getenv("ANDROIDQF_FAKE_ADB_SHELL_FAIL") == "1" {
			os.Exit(1)
		}
	default:
		os.Exit(2)
	}
}

func TestExecCancelsActiveADBCommand(t *testing.T) {
	client := newFakeADB(t, "")
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("ANDROIDQF_FAKE_ADB_MARKER", marker)
	ctx, cancel := context.WithCancel(context.Background())
	client.SetContext(ctx)

	done := make(chan error, 1)
	go func() {
		_, err := client.Exec("wait")
		done <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake adb did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Exec() error = nil after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Exec() did not stop after cancellation")
	}
}

func TestListFilesReturnsPartialOutputAndError(t *testing.T) {
	client := newFakeADB(t, "")
	t.Setenv("ANDROIDQF_FAKE_ADB_REQUIRE_TYPE_FILE", "1")
	t.Setenv("ANDROIDQF_FAKE_ADB_SHELL_OUTPUT", "/sdcard/one\n/sdcard/two\n")
	t.Setenv("ANDROIDQF_FAKE_ADB_SHELL_FAIL", "1")

	files, err := client.ListFiles("/sdcard", true)
	if err == nil {
		t.Fatal("ListFiles() error = nil")
	}
	if len(files) != 2 || files[0] != "/sdcard/one" || files[1] != "/sdcard/two" {
		t.Fatalf("ListFiles() files = %v", files)
	}
}

func TestDeviceInfosParsesLongDeviceList(t *testing.T) {
	client := newFakeADB(t, "device-1,device-2")
	devices, err := client.DeviceInfos()
	if err != nil {
		t.Fatalf("DeviceInfos returned error: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("DeviceInfos returned %d devices, want 2", len(devices))
	}
	if devices[0].Serial != "device-1" {
		t.Fatalf("first serial = %q, want device-1", devices[0].Serial)
	}
	if devices[0].Model != "device-1_Model" {
		t.Fatalf("first model = %q, want device-1_Model", devices[0].Model)
	}
}

func TestParseDeviceInfoLineUnauthorizedWithoutModel(t *testing.T) {
	info, ok := parseDeviceInfoLine("5B221JEBF18336         unauthorized usb:336592896X transport_id:1")
	if !ok {
		t.Fatal("parseDeviceInfoLine returned ok=false")
	}
	if info.Serial != "5B221JEBF18336" {
		t.Fatalf("serial = %q, want 5B221JEBF18336", info.Serial)
	}
	if info.State != "unauthorized" {
		t.Fatalf("state = %q, want unauthorized", info.State)
	}
	if info.Model != "" {
		t.Fatalf("model = %q, want empty", info.Model)
	}
}

func newFakeADB(t *testing.T, devices string) *ADB {
	t.Helper()
	t.Setenv("ANDROIDQF_FAKE_ADB", "1")
	t.Setenv("ANDROIDQF_FAKE_ADB_DEVICES", devices)
	return &ADB{ExePath: os.Args[0]}
}

func TestSetSerialSingleDeviceUsesExplicitSerial(t *testing.T) {
	client := newFakeADB(t, "device-1")
	serial, err := client.SetSerial("")
	if err != nil {
		t.Fatalf("SetSerial returned error: %v", err)
	}
	if serial != "device-1" {
		t.Fatalf("serial = %q, want device-1", serial)
	}
	if client.Serial != "device-1" {
		t.Fatalf("client.Serial = %q, want device-1", client.Serial)
	}
}

func TestSetSerialMultipleDevicesWithoutSerialErrors(t *testing.T) {
	client := newFakeADB(t, "device-1,device-2")
	_, err := client.SetSerial("")
	if err == nil {
		t.Fatal("SetSerial returned nil error, want multiple devices error")
	}
}

func TestSetSerialExplicitSerial(t *testing.T) {
	client := newFakeADB(t, "device-1,device-2")
	serial, err := client.SetSerial("device-2")
	if err != nil {
		t.Fatalf("SetSerial returned error: %v", err)
	}
	if serial != "device-2" {
		t.Fatalf("serial = %q, want device-2", serial)
	}
}
