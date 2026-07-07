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
				fmt.Printf("%s\tdevice\n", device)
			}
		}
	default:
		os.Exit(2)
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
