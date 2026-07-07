package main

import (
	"os"
	"testing"
)

func TestRegisterRunningExtractionAppearsActiveAndReleases(t *testing.T) {
	stateDir := t.TempDir()
	oldStateDir := runningStateDir
	oldProcessExists := processExists
	runningStateDir = func() string { return stateDir }
	processExists = func(pid int) bool { return pid == os.Getpid() }
	t.Cleanup(func() {
		runningStateDir = oldStateDir
		processExists = oldProcessExists
	})

	release, err := registerRunningExtraction("device-1", "out")
	if err != nil {
		t.Fatalf("registerRunningExtraction returned error: %v", err)
	}

	active := activeRunningExtractionsBySerial()
	state, ok := active["device-1"]
	if !ok {
		t.Fatal("device-1 was not found in active running extractions")
	}
	if state.StoragePath != "out" {
		t.Fatalf("storage path = %q, want out", state.StoragePath)
	}

	release()
	active = activeRunningExtractionsBySerial()
	if _, ok := active["device-1"]; ok {
		t.Fatal("device-1 remained active after release")
	}
}

func TestActiveRunningExtractionsRemovesStaleState(t *testing.T) {
	stateDir := t.TempDir()
	oldStateDir := runningStateDir
	oldProcessExists := processExists
	runningStateDir = func() string { return stateDir }
	processExists = func(int) bool { return false }
	t.Cleanup(func() {
		runningStateDir = oldStateDir
		processExists = oldProcessExists
	})

	release, err := registerRunningExtraction("device-1", "out")
	if err != nil {
		t.Fatalf("registerRunningExtraction returned error: %v", err)
	}
	defer release()

	active := activeRunningExtractionsBySerial()
	if len(active) != 0 {
		t.Fatalf("active state count = %d, want 0", len(active))
	}

	entries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("state files remaining = %d, want 0", len(entries))
	}
}
