package main

import (
	"errors"
	"testing"
	"time"
)

func TestResolveADBSerialNoDevicesDoesNotPrompt(t *testing.T) {
	called := false
	serial, prompted, err := resolveADBSerial("", nil, func([]deviceMenuItem) (string, error) {
		called = true
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("resolveADBSerial returned error: %v", err)
	}
	if serial != "" {
		t.Fatalf("serial = %q, want empty", serial)
	}
	if prompted {
		t.Fatal("prompted = true, want false")
	}
	if called {
		t.Fatal("selector was called for zero devices")
	}
}

func TestResolveADBSerialSingleDeviceDoesNotPrompt(t *testing.T) {
	called := false
	serial, prompted, err := resolveADBSerial("", []string{"device-1"}, func([]deviceMenuItem) (string, error) {
		called = true
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("resolveADBSerial returned error: %v", err)
	}
	if serial != "device-1" {
		t.Fatalf("serial = %q, want device-1", serial)
	}
	if prompted {
		t.Fatal("prompted = true, want false")
	}
	if called {
		t.Fatal("selector was called for one device")
	}
}

func TestResolveADBSerialMultipleDevicesPromptsWithRunningStatus(t *testing.T) {
	started := time.Date(2026, 7, 7, 10, 11, 12, 0, time.UTC)
	running := map[string]runningExtraction{
		"device-2": {
			Serial:  "device-2",
			PID:     1234,
			Started: started,
		},
	}

	var gotItems []deviceMenuItem
	serial, prompted, err := resolveADBSerial("", []string{"device-1", "device-2"}, func(items []deviceMenuItem) (string, error) {
		gotItems = items
		return items[1].Serial, nil
	}, running)
	if err != nil {
		t.Fatalf("resolveADBSerial returned error: %v", err)
	}
	if serial != "device-2" {
		t.Fatalf("serial = %q, want device-2", serial)
	}
	if !prompted {
		t.Fatal("prompted = false, want true")
	}
	if len(gotItems) != 2 {
		t.Fatalf("selector got %d items, want 2", len(gotItems))
	}
	if gotItems[0].Status != "" {
		t.Fatalf("first item status = %q, want empty", gotItems[0].Status)
	}
	if gotItems[1].Status == "" {
		t.Fatal("second item status is empty, want running extraction status")
	}
}

func TestResolveADBSerialMultipleDevicesReturnsSelectorError(t *testing.T) {
	wantErr := errors.New("selection failed")
	serial, prompted, err := resolveADBSerial("", []string{"device-1", "device-2"}, func([]deviceMenuItem) (string, error) {
		return "", wantErr
	}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if serial != "" {
		t.Fatalf("serial = %q, want empty", serial)
	}
	if !prompted {
		t.Fatal("prompted = false, want true")
	}
}

func TestResolveADBSerialExplicitSerialDoesNotPrompt(t *testing.T) {
	called := false
	serial, prompted, err := resolveADBSerial("requested", []string{"device-1", "device-2"}, func([]deviceMenuItem) (string, error) {
		called = true
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("resolveADBSerial returned error: %v", err)
	}
	if serial != "requested" {
		t.Fatalf("serial = %q, want requested", serial)
	}
	if prompted {
		t.Fatal("prompted = true, want false")
	}
	if called {
		t.Fatal("selector was called for explicit serial")
	}
}
