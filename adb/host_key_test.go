package adb

import (
	"os"
	"path/filepath"
	"testing"
)

const testADBPublicKey = "AAAA-test-adb-public-key user@host"

func TestHostPublicKeyReadsPublicKeyFile(t *testing.T) {
	privateKeyPath := filepath.Join(t.TempDir(), "adbkey")
	if err := os.WriteFile(privateKeyPath+".pub", []byte(testADBPublicKey+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(public key) error = %v", err)
	}

	client := &ADB{}
	key, err := client.hostPublicKey(privateKeyPath)
	if err != nil {
		t.Fatalf("hostPublicKey() error = %v", err)
	}
	if key != testADBPublicKey {
		t.Fatalf("hostPublicKey() = %q, want %q", key, testADBPublicKey)
	}
}

func TestHostPublicKeyDerivesMissingPublicKey(t *testing.T) {
	client := newFakeADB(t, "")
	t.Setenv("ANDROIDQF_FAKE_ADB_PUBLIC_KEY", testADBPublicKey)

	key, err := client.hostPublicKey(filepath.Join(t.TempDir(), "adbkey"))
	if err != nil {
		t.Fatalf("hostPublicKey() error = %v", err)
	}
	if key != testADBPublicKey {
		t.Fatalf("hostPublicKey() = %q, want %q", key, testADBPublicKey)
	}
}

func TestNormalizeHostPublicKeyRejectsMultipleLines(t *testing.T) {
	if _, err := normalizeHostPublicKey([]byte("first\nsecond\n"), "test"); err == nil {
		t.Fatal("normalizeHostPublicKey() error = nil, want multiline error")
	}
}
