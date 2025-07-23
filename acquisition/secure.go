// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/botherder/go-savetime/files"
	saveRuntime "github.com/botherder/go-savetime/runtime"
	"github.com/mvt-project/androidqf/log"
)

func (a *Acquisition) StoreSecurely() error {
	cwd := saveRuntime.GetExecutableDirectory()

	keyFilePath := filepath.Join(cwd, "key.txt")
	if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
		return nil
	}

	log.Info("You provided an age public key, storing the acquisition securely.")

	zipFileName := fmt.Sprintf("%s.zip", a.UUID)
	zipFilePath := filepath.Join(cwd, zipFileName)

	log.Info("Compressing the acquisition folder. This might take a while...")

	err := files.Zip(a.StoragePath, zipFilePath)
	if err != nil {
		return err
	}

	log.Info("Encrypting the compressed archive. This might take a while...")

	publicKey, err := os.ReadFile(keyFilePath)
	if err != nil {
		return err
	}
	publicKeyStr := strings.TrimSpace(string(publicKey))

	recipient, err := age.ParseX25519Recipient(publicKeyStr)
	if err != nil {
		return fmt.Errorf("failed to parse public key %q: %v", publicKeyStr, err)
	}

	zipFile, err := os.Open(zipFilePath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	encFileName := fmt.Sprintf("%s.age", zipFileName)
	encFilePath := filepath.Join(cwd, encFileName)
	encFile, err := os.OpenFile(encFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("unable to create encrypted file: %v", err)
	}
	defer encFile.Close()

	w, err := age.Encrypt(encFile, recipient)
	if err != nil {
		return fmt.Errorf("failed to create encrypted file: %v", err)
	}

	_, err = io.Copy(w, zipFile)
	if err != nil {
		return fmt.Errorf("failed to write to encrypted file: %v", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close encrypted file: %v", err)
	}

	log.Infof("Acquisition successfully encrypted at %s", encFilePath)

	// TODO: we should securely wipe the files.
	zipFile.Close()
	err = os.Remove(zipFilePath)
	if err != nil {
		return fmt.Errorf("failed to delete the unencrypted compressed archive: %v", err)
	}

	// Ensure log file is closed before removing the acquisition directory
	if a.closeLog != nil {
		defer a.closeLog()
	}

	err = os.RemoveAll(a.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to delete the original unencrypted acquisition folder: %v", err)
	}

	return nil
}
