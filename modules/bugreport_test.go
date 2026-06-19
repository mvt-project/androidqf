// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import "testing"

func TestOldBugreportArchiveName(t *testing.T) {
	tests := []struct {
		name       string
		devicePath string
		want       string
		wantOK     bool
	}{
		{
			name:       "bugreport zip",
			devicePath: "/bugreports/bugreport-bluejay-AP2A.240905.003.F1.zip",
			want:       "bugreport-bluejay-AP2A.240905.003.F1.zip",
			wantOK:     true,
		},
		{
			name:       "plain bugreport zip",
			devicePath: "/data/user_de/0/com.android.shell/files/bugreport.zip",
			want:       "bugreport.zip",
			wantOK:     true,
		},
		{
			name:       "directory ignored",
			devicePath: "/bugreports/",
			wantOK:     false,
		},
		{
			name:       "unrelated shell file ignored",
			devicePath: "/data/user_de/0/com.android.shell/files/trace.txt",
			wantOK:     false,
		},
		{
			name:       "nul rejected",
			devicePath: "/bugreports/bugreport-bad.zip\x00",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := oldBugreportArchiveName(tt.devicePath)
			if ok != tt.wantOK {
				t.Fatalf("oldBugreportArchiveName() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("oldBugreportArchiveName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUniqueOldBugreportFilesDeduplicatesAliases(t *testing.T) {
	files := uniqueOldBugreportFiles([]string{
		"/bugreports/bugreport-bluejay-AP2A.240905.003.F1.zip",
		"/data/user_de/0/com.android.shell/files/bugreports/bugreport-bluejay-AP2A.240905.003.F1.zip",
		"/data/user_de/0/com.android.shell/files/bugreports/bugreport-a54xnsxx-TP1A.220624.014.zip",
		"/data/user_de/0/com.android.shell/files/not-a-bugreport.txt",
	})

	if len(files) != 2 {
		t.Fatalf("uniqueOldBugreportFiles() length = %d, want 2: %#v", len(files), files)
	}
	if files[0].ArchiveName != "bugreport-bluejay-AP2A.240905.003.F1.zip" {
		t.Fatalf("first archive name = %q", files[0].ArchiveName)
	}
	if files[0].DevicePath != "/bugreports/bugreport-bluejay-AP2A.240905.003.F1.zip" {
		t.Fatalf("first device path = %q", files[0].DevicePath)
	}
	if files[1].ArchiveName != "bugreport-a54xnsxx-TP1A.220624.014.zip" {
		t.Fatalf("second archive name = %q", files[1].ArchiveName)
	}
}
