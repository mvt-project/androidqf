package main

import (
	"os"
	"testing"
)

func TestRegisterRunningExtractionAppearsActiveAndReleases(t *testing.T) {
	stateDir := t.TempDir()
	oldStateDir := runningStateDir
	oldProcessExists := processExists
	oldProcessToken := processToken
	runningStateDir = func() string { return stateDir }
	processExists = func(pid int) bool { return pid == os.Getpid() }
	processToken = func(pid int) string { return "start-token" }
	t.Cleanup(func() {
		runningStateDir = oldStateDir
		processExists = oldProcessExists
		processToken = oldProcessToken
	})

	release, err := registerRunningExtraction("device-1", "out")
	if err != nil {
		t.Fatalf("registerRunningExtraction returned error: %v", err)
	}
	dirInfo, err := os.Stat(stateDir)
	if err != nil {
		t.Fatalf("Stat(stateDir) error = %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("state directory mode = %o, want 700", got)
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("state entries = %v, error = %v", entries, err)
	}
	fileInfo, err := entries[0].Info()
	if err != nil {
		t.Fatalf("state file Info() error = %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("state file mode = %o, want 600", got)
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
	oldProcessToken := processToken
	runningStateDir = func() string { return stateDir }
	processExists = func(int) bool { return false }
	processToken = func(int) string { return "start-token" }
	t.Cleanup(func() {
		runningStateDir = oldStateDir
		processExists = oldProcessExists
		processToken = oldProcessToken
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

func TestActiveRunningExtractionsRemovesReusedPIDState(t *testing.T) {
	stateDir := t.TempDir()
	oldStateDir := runningStateDir
	oldProcessExists := processExists
	oldProcessToken := processToken
	runningStateDir = func() string { return stateDir }
	processExists = func(int) bool { return true }
	processToken = func(int) string { return "original-token" }
	t.Cleanup(func() {
		runningStateDir = oldStateDir
		processExists = oldProcessExists
		processToken = oldProcessToken
	})

	release, err := registerRunningExtraction("device-1", "out")
	if err != nil {
		t.Fatalf("registerRunningExtraction returned error: %v", err)
	}
	defer release()
	processToken = func(int) string { return "reused-pid-token" }

	if active := activeRunningExtractionsBySerial(); len(active) != 0 {
		t.Fatalf("active state = %+v, want none", active)
	}
}
