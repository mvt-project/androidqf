// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"testing"
)

func TestRelativeDeviceChild(t *testing.T) {
	tests := []struct {
		name       string
		deviceRoot string
		devicePath string
		want       string
		wantErr    bool
	}{
		{
			name:       "child with trailing slash root",
			deviceRoot: "/sdcard/Download/Intrusion Logging/",
			devicePath: "/sdcard/Download/Intrusion Logging/logs/file.txt",
			want:       "logs/file.txt",
		},
		{
			name:       "child without trailing slash root",
			deviceRoot: "/data/local/tmp",
			devicePath: "/data/local/tmp/file.txt",
			want:       "file.txt",
		},
		{
			name:       "sibling prefix rejected",
			deviceRoot: "/data/local/tmp",
			devicePath: "/data/local/tmp-evil/file.txt",
			wantErr:    true,
		},
		{
			name:       "parent traversal rejected",
			deviceRoot: "/data/local/tmp",
			devicePath: "/data/local/tmp/../../../host/path",
			wantErr:    true,
		},
		{
			name:       "cleaned child traversal rejected",
			deviceRoot: "/sdcard/Download/Intrusion Logging/",
			devicePath: "/sdcard/Download/Intrusion Logging/../Other/file.txt",
			wantErr:    true,
		},
		{
			name:       "non child rejected",
			deviceRoot: "/sdcard/Download/Intrusion Logging/",
			devicePath: "/sdcard/Download/Other/file.txt",
			wantErr:    true,
		},
		{
			name:       "root rejected",
			deviceRoot: "/data/local/tmp/",
			devicePath: "/data/local/tmp",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := relativeDeviceChild(tt.deviceRoot, tt.devicePath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("relativeDeviceChild() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("relativeDeviceChild() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("relativeDeviceChild() = %q, want %q", got, tt.want)
			}
		})
	}
}
