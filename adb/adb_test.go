package adb

import (
	"fmt"
	"os"
	"strings"
	"testing"
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
	default:
		os.Exit(2)
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
