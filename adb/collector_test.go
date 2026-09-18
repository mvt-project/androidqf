package adb

import "testing"

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
