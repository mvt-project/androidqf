package acquisition

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindAgeKeyFileInDirsPrefersCurrentWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	executableDir := t.TempDir()

	cwdKey := filepath.Join(cwd, keyFileName)
	executableKey := filepath.Join(executableDir, keyFileName)
	if err := os.WriteFile(cwdKey, []byte("cwd"), 0o600); err != nil {
		t.Fatalf("WriteFile(cwd key) error = %v", err)
	}
	if err := os.WriteFile(executableKey, []byte("exe"), 0o600); err != nil {
		t.Fatalf("WriteFile(executable key) error = %v", err)
	}

	got, ok := findAgeKeyFileInDirs(cwd, executableDir)
	if !ok {
		t.Fatal("findAgeKeyFileInDirs() did not find a key")
	}
	if got != cwdKey {
		t.Fatalf("key path = %q, want %q", got, cwdKey)
	}
}

func TestFindAgeKeyFileInDirsFallsBackToExecutableDirectory(t *testing.T) {
	cwd := t.TempDir()
	executableDir := t.TempDir()

	executableKey := filepath.Join(executableDir, keyFileName)
	if err := os.WriteFile(executableKey, []byte("exe"), 0o600); err != nil {
		t.Fatalf("WriteFile(executable key) error = %v", err)
	}

	got, ok := findAgeKeyFileInDirs(cwd, executableDir)
	if !ok {
		t.Fatal("findAgeKeyFileInDirs() did not find a key")
	}
	if got != executableKey {
		t.Fatalf("key path = %q, want %q", got, executableKey)
	}
}
