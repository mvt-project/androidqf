// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type runningExtraction struct {
	Serial      string    `json:"serial"`
	PID         int       `json:"pid"`
	Started     time.Time `json:"started"`
	StoragePath string    `json:"storage_path,omitempty"`
}

var (
	runningStateDir = defaultRunningStateDir
	processExists   = defaultProcessExists
)

func defaultRunningStateDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "androidqf", "running")
	}
	return filepath.Join(cacheDir, "androidqf", "running")
}

func runningExtractionFileName(pid int, serial string) string {
	encodedSerial := base64.RawURLEncoding.EncodeToString([]byte(serial))
	return fmt.Sprintf("%d-%s.json", pid, encodedSerial)
}

func registerRunningExtraction(serial, storagePath string) (func(), error) {
	if strings.TrimSpace(serial) == "" {
		return func() {}, nil
	}

	stateDir := runningStateDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}

	state := runningExtraction{
		Serial:      serial,
		PID:         os.Getpid(),
		Started:     time.Now().UTC(),
		StoragePath: storagePath,
	}
	statePath := filepath.Join(stateDir, runningExtractionFileName(state.PID, state.Serial))

	data, err := json.MarshalIndent(state, "", " ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		return nil, err
	}

	return func() {
		_ = os.Remove(statePath)
	}, nil
}

func activeRunningExtractionsBySerial() map[string]runningExtraction {
	result := make(map[string]runningExtraction)
	stateDir := runningStateDir()

	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		statePath := filepath.Join(stateDir, entry.Name())
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}

		var state runningExtraction
		if err := json.Unmarshal(data, &state); err != nil || state.Serial == "" || state.PID == 0 {
			_ = os.Remove(statePath)
			continue
		}

		if !processExists(state.PID) {
			_ = os.Remove(statePath)
			continue
		}

		result[state.Serial] = state
	}

	return result
}

func defaultProcessExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pid == os.Getpid() {
		return true
	}

	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), fmt.Sprintf("\"%d\"", pid))
	}

	return exec.Command("kill", "-0", strconv.Itoa(pid)).Run() == nil
}
