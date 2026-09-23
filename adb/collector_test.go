package adb

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvt-project/androidqf/assets"
)

func TestCollectorNameForArchitecture(t *testing.T) {
	tests := []struct {
		architecture string
		want         string
	}{
		{architecture: "armeabi-v7a", want: "collector_arm"},
		{architecture: "arm64-v8a", want: "collector_arm64"},
		{architecture: "x86_64", want: "collector_amd64"},
	}
	for _, tt := range tests {
		t.Run(tt.architecture, func(t *testing.T) {
			got, err := collectorNameForArchitecture(tt.architecture)
			if err != nil {
				t.Fatalf("collectorNameForArchitecture() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("collectorNameForArchitecture() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectorNameForArchitectureRejectsUnsupportedABI(t *testing.T) {
	if _, err := collectorNameForArchitecture("x86"); err == nil {
		t.Fatal("collectorNameForArchitecture(x86) error = nil")
	}
}

func TestGetCollectorRecordsSelectedBinaryHash(t *testing.T) {
	for _, tt := range []struct {
		architecture string
		asset        string
	}{
		{architecture: "armeabi-v7a", asset: "collector_arm"},
		{architecture: "arm64-v8a", asset: "collector_arm64"},
		{architecture: "x86_64", asset: "collector_amd64"},
	} {
		t.Run(tt.architecture, func(t *testing.T) {
			client := newFakeADB(t, "")
			t.Setenv("ANDROIDQF_FAKE_ADB_SHELL_OUTPUT", "1") // FileExists reports absent.
			pushedPath := filepath.Join(t.TempDir(), "collector")
			t.Setenv("ANDROIDQF_FAKE_ADB_PUSH_COPY", pushedPath)

			collector, err := client.GetCollector("/data/local/tmp", tt.architecture)
			if err != nil {
				t.Fatalf("GetCollector() error = %v", err)
			}
			pushed, err := os.ReadFile(pushedPath)
			if err != nil {
				t.Fatalf("ReadFile(pushed collector) error = %v", err)
			}
			bundled, err := assets.ReadCollectorFile(tt.asset)
			if err != nil {
				t.Fatalf("ReadCollectorFile(%q) error = %v", tt.asset, err)
			}
			if !bytes.Equal(pushed, bundled) {
				t.Fatal("pushed collector differs from selected asset")
			}
			want := fmt.Sprintf("%x", sha256.Sum256(pushed))
			if collector.SHA256 != want {
				t.Fatalf("collector.SHA256 = %q, want %q", collector.SHA256, want)
			}

			metadata, err := json.Marshal(struct {
				Collector *Collector `json:"collector"`
			}{Collector: collector})
			if err != nil {
				t.Fatalf("json.Marshal(collector metadata) error = %v", err)
			}
			var saved struct {
				Collector struct {
					SHA256 string `json:"sha256"`
				} `json:"collector"`
			}
			if err := json.Unmarshal(metadata, &saved); err != nil {
				t.Fatalf("json.Unmarshal(collector metadata) error = %v", err)
			}
			if saved.Collector.SHA256 != want {
				t.Fatalf("collector.sha256 in acquisition metadata = %q, want %q", saved.Collector.SHA256, want)
			}
		})
	}
}

func TestCollectorInstallFailureClearsHash(t *testing.T) {
	client := newFakeADB(t, "")
	t.Setenv("ANDROIDQF_FAKE_ADB_SHELL_OUTPUT", "1")
	t.Setenv("ANDROIDQF_FAKE_ADB_PUSH_FAIL", "1")
	collector := &Collector{
		ExePath:      "/data/local/tmp/collector",
		Adb:          client,
		Architecture: "arm64-v8a",
		SHA256:       "previous-install-hash",
	}
	if err := collector.Install(); err == nil {
		t.Fatal("Install() error = nil, want push failure")
	}
	if collector.SHA256 != "" {
		t.Fatalf("collector.SHA256 = %q after failed install, want empty", collector.SHA256)
	}
}
