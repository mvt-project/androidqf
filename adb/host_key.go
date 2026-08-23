// androidqf - Android Quick Forensics
// Copyright (c) 2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
// https://license.mvt.re/1.1/

package adb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HostPublicKey returns the public half of ADB's per-user host key. The private
// key must never be copied into an acquisition archive.
func (a *ADB) HostPublicKey() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to find user home directory: %w", err)
	}

	return a.hostPublicKey(filepath.Join(homeDir, ".android", "adbkey"))
}

func (a *ADB) hostPublicKey(privateKeyPath string) (string, error) {
	publicKeyPath := privateKeyPath + ".pub"
	publicKey, err := os.ReadFile(publicKeyPath)
	if err == nil {
		return normalizeHostPublicKey(publicKey, publicKeyPath)
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read ADB host public key: %w", err)
	}

	// ADB normally creates adbkey.pub alongside adbkey. Derive it from the
	// private key if the public file is missing, without exposing private data.
	publicKey, err = a.command("pubkey", privateKeyPath).Output()
	if err != nil {
		return "", fmt.Errorf("failed to derive ADB host public key: %w", err)
	}
	return normalizeHostPublicKey(publicKey, publicKeyPath)
}

func normalizeHostPublicKey(publicKey []byte, source string) (string, error) {
	key := strings.TrimSpace(string(publicKey))
	if key == "" {
		return "", fmt.Errorf("ADB host public key from %s is empty", source)
	}
	if strings.ContainsAny(key, "\r\n") {
		return "", fmt.Errorf("ADB host public key from %s contains multiple lines", source)
	}
	return key, nil
}
