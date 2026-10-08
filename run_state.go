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
	Serial       string    `json:"serial"`
	PID          int       `json:"pid"`
	ProcessToken string    `json:"process_token"`
	Started      time.Time `json:"started"`
	StoragePath  string    `json:"storage_path,omitempty"`
}

var (
	runningStateDir = defaultRunningStateDir
	processExists   = defaultProcessExists
	processToken    = defaultProcessToken
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
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(stateDir, 0o700); err != nil {
		return nil, err
	}

	state := runningExtraction{
		Serial:       serial,
		PID:          os.Getpid(),
		ProcessToken: processToken(os.Getpid()),
		Started:      time.Now().UTC(),
		StoragePath:  storagePath,
	}

	data, err := json.MarshalIndent(state, "", " ")
	if err != nil {
		return nil, err
	}
	tempFile, err := os.CreateTemp(stateDir, runningExtractionFileName(state.PID, state.Serial)+"-*.tmp")
	if err != nil {
		return nil, err
	}
	tempPath := tempFile.Name()
	cleanupTemp := func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}
	if _, err := tempFile.Write(data); err != nil {
		cleanupTemp()
		return nil, err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	statePath := strings.TrimSuffix(tempPath, ".tmp") + ".json"
	if err := os.Rename(tempPath, statePath); err != nil {
		_ = os.Remove(tempPath)
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

		if !processExists(state.PID) || state.ProcessToken == "" || processToken(state.PID) != state.ProcessToken {
			_ = os.Remove(statePath)
			continue
		}

		result[state.Serial] = state
	}

	return result
}

func defaultProcessToken(pid int) string {
	if pid <= 0 {
		return ""
	}
	if runtime.GOOS == "windows" {
		command := fmt.Sprintf("(Get-Process -Id %d).StartTime.ToUniversalTime().Ticks", pid)
		out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		if err != nil {
			return ""
		}
		stat := string(data)
		close := strings.LastIndex(stat, ")")
		if close < 0 {
			return ""
		}
		fields := strings.Fields(stat[close+1:])
		if len(fields) <= 19 {
			return ""
		}
		return fields[19]
	}

	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
