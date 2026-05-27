// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

func TestCreateRootFile(t *testing.T) {
	rootDir := t.TempDir()
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatalf("OpenRoot() error = %v", err)
	}
	defer root.Close()

	file, err := createRootFile(root, "nested/file.txt")
	if err != nil {
		t.Fatalf("createRootFile() error = %v", err)
	}
	if _, err := file.WriteString("ok"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(rootDir, "nested", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("created file content = %q, want %q", got, "ok")
	}

	file, err = createRootFile(root, "file.txt")
	if err != nil {
		t.Fatalf("createRootFile() root file error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() root file error = %v", err)
	}

	if file, err := createRootFile(root, "../escape"); err == nil {
		file.Close()
		t.Fatal("createRootFile() error = nil, want lexical traversal rejection")
	}
}

func TestCreateRootFileRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on Windows")
	}

	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, filepath.Join(rootDir, "escape")); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation not permitted: %v", err)
		}
		t.Fatalf("Symlink() error = %v", err)
	}

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatalf("OpenRoot() error = %v", err)
	}
	defer root.Close()

	if file, err := createRootFile(root, "escape/file.txt"); err == nil {
		file.Close()
		t.Fatal("createRootFile() error = nil, want symlink escape rejection")
	}
}
