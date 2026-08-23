// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/avast/apkverifier"
	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
	"github.com/mvt-project/androidqf/utils"
)

const (
	apkAll           = "All"
	apkNotSystem     = "Only non-system packages"
	apkNone          = "Do not download any"
	apkRemoveTrusted = "Yes"
	apkKeepAll       = "No"
)

type Packages struct{}

func NewPackages() *Packages {
	return &Packages{}
}

func (p *Packages) Name() string {
	return "packages"
}

func (p *Packages) extractFileName(filePath string) (string, error) {
	if !strings.Contains(filePath, "==/") {
		return "", nil
	}
	parts := strings.Split(filePath, "==/")
	if len(parts) <= 1 {
		return "", nil
	}
	raw := strings.Replace(parts[1], ".apk", "", 1)
	if raw == "" {
		return "", nil
	}
	if !filepath.IsLocal(raw) {
		return "", fmt.Errorf("non-local APK path component: %q", raw)
	}
	return fmt.Sprintf("_%s", raw), nil
}

func (p *Packages) generateZipPath(packageName, filePath string) (string, error) {
	suffix, err := p.extractFileName(filePath)
	if err != nil {
		return "", err
	}
	base := fmt.Sprintf("%s%s.apk", packageName, suffix)
	if !filepath.IsLocal(base) {
		return "", fmt.Errorf("non-local zip entry basename: %q", base)
	}
	return "apks/" + base, nil
}

func reserveUniqueZipPath(zipPath string, used map[string]struct{}) string {
	if used == nil {
		return zipPath
	}
	if _, ok := used[zipPath]; !ok {
		used[zipPath] = struct{}{}
		return zipPath
	}

	ext := filepath.Ext(zipPath)
	base := strings.TrimSuffix(zipPath, ext)
	for counter := 1; ; counter++ {
		candidate := fmt.Sprintf("%s_%d%s", base, counter, ext)
		if _, ok := used[candidate]; !ok {
			used[candidate] = struct{}{}
			return candidate
		}
	}
}

func ParseDownloadOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "all":
		return apkAll, nil
	case "non-system":
		return apkNotSystem, nil
	case "none":
		return apkNone, nil
	}
	return "", fmt.Errorf("invalid -download value %q (valid values: all, non-system, none)", value)
}

func ParseRemoveTrustedOption(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes":
		return apkRemoveTrusted, nil
	case "no":
		return apkKeepAll, nil
	}
	return "", fmt.Errorf("invalid -remove-trusted value %q (valid values: yes, no)", value)
}

func (p *Packages) Run(acq *acquisition.Acquisition, opts *Options) error {
	log.Info("Collecting information on installed apps. This might take a while...")
	var collectionErr error

	packages, err := adb.Client.GetPackages(opts.Fast)
	if err != nil {
		return fmt.Errorf("failed to retrieve list of installed packages: %v", err)
	}

	log.Infof(
		"Found a total of %d installed packages",
		len(packages),
	)

	download, err := resolveOption(opts, opts.Download, "-download (all, non-system, none)", func() (string, error) {
		log.Info("Would you like to download copies of all apps or only non-system ones?")
		downloadPrompt := promptui.Select{
			Label: "Download",
			Items: []string{apkAll, apkNotSystem, apkNone},
		}
		_, selection, err := downloadPrompt.Run()
		return selection, err
	})
	if err != nil {
		return fmt.Errorf("failed to make selection for download option: %v", err)
	}

	// If the user decides to not download any APK, then we skip this.
	// Otherwise we walk through the list of package, pull the files, and hash them.
	if download != apkNone {

		var keepOption string

		keepOption, err = resolveOption(opts, opts.RemoveTrusted, "-remove-trusted (yes, no)", func() (string, error) {
			log.Info("Would you like to remove copies of apps signed with a trusted certificate to limit the size of the output archive?")
			promptAll := promptui.Select{
				Label: "Remove",
				Items: []string{apkRemoveTrusted, apkKeepAll},
			}
			_, selection, err := promptAll.Run()
			return selection, err
		})
		if err != nil {
			return fmt.Errorf("failed to make selection for download option: %v",
				err)
		}

		usedZipPaths := make(map[string]struct{})
		for ip := 0; ip < len(packages); ip++ {
			// If we the user did not request to download all packages and if
			// the package is marked as system, we skip it.
			if download != apkAll && packages[ip].System {
				continue
			}

			log.Debugf("Found Android package: %s", packages[ip].Name)

			for ipf := 0; ipf < len(packages[ip].Files); ipf++ {
				packageFile := &packages[ip].Files[ipf]

				if err := p.processAPKStreaming(packages[ip].Name, packageFile, keepOption, acq, usedZipPaths); err != nil {
					log.Debugf("ERROR: failed to process APK %s: %v", packageFile.Path, err)
					collectionErr = errors.Join(collectionErr, fmt.Errorf("%s: %w", packageFile.Path, err))
					continue
				}
			}
		}
	}

	saveErr := saveDataToAcquisition(acq, "packages.json", &packages)
	if saveErr != nil {
		return errors.Join(collectionErr, saveErr)
	}
	return partialCollectionError(collectionErr)
}

func (p *Packages) processAPKStreaming(packageName string, packageFile *adb.PackageFile, keepOption string, acq *acquisition.Acquisition, usedZipPaths map[string]struct{}) error {
	zipPath, err := p.generateZipPath(packageName, packageFile.Path)
	if err != nil {
		log.Errorf("Skipping APK with unsafe path %q: %v", packageFile.Path, err)
		packageFile.Error = err.Error()
		return err
	}

	buffer, err := acq.StreamingPuller.PullToBuffer(packageFile.Path)
	if err != nil {
		if errors.Is(err, acquisition.ErrStreamingBufferMemoryLimit) {
			if acq.ZipWriter != nil && acq.ZipWriter.IsEncrypted() {
				zipPath, skipped, err := p.processLargeEncryptedAPK(packageFile, keepOption, zipPath, acq, usedZipPaths)
				if err != nil {
					packageFile.Error = err.Error()
					return err
				}
				if skipped {
					log.Debugf("Trusted APK skipped for streaming: %s", packageFile.Path)
					return nil
				}
				log.Debugf("Streamed %s directly to archive as %s", packageFile.Path, zipPath)
				return nil
			}

			zipPath, skipped, err := p.processLargeAPKFromTemp(packageFile, keepOption, zipPath, acq, usedZipPaths)
			if err != nil {
				packageFile.Error = err.Error()
				return err
			}
			if skipped {
				log.Debugf("Trusted APK skipped for streaming: %s", packageFile.Path)
				return nil
			}
			log.Debugf("Streamed %s directly to archive as %s", packageFile.Path, zipPath)
			return nil
		}
		packageFile.Error = fmt.Sprintf("Failed to pull APK: %v", err)
		return err
	}

	shouldSkip, err := p.processCertificate(packageFile, keepOption, buffer)
	if err != nil {
		packageFile.Error = fmt.Sprintf("Certificate processing failed: %v", err)
		return err
	}
	if shouldSkip {
		log.Debugf("Trusted APK skipped for streaming: %s", packageFile.Path)
		return nil
	}
	zipPath = reserveUniqueZipPath(zipPath, usedZipPaths)
	err = acq.ZipWriter.CreateFileFromReader(zipPath, buffer.Reader())
	if err != nil {
		packageFile.Error = fmt.Sprintf("Failed to stream to archive: %v", err)
		return err
	}

	log.Debugf("Streamed %s directly to archive as %s", packageFile.Path, zipPath)
	return nil
}

func (p *Packages) processLargeEncryptedAPK(packageFile *adb.PackageFile, keepOption, zipPath string, acq *acquisition.Acquisition, usedZipPaths map[string]struct{}) (string, bool, error) {
	log.Debugf("APK %s exceeded streaming buffer limit; using encrypted random-access staging for certificate verification", packageFile.Path)

	staged, err := acq.StreamingPuller.PullToEncryptedTempFile(packageFile.Path)
	if err != nil {
		return "", false, fmt.Errorf("failed to pull APK to encrypted temporary file: %w", err)
	}
	defer staged.Remove()

	reader, err := staged.Open()
	if err != nil {
		return "", false, fmt.Errorf("failed to open encrypted temporary APK: %w", err)
	}
	shouldSkip, certErr := p.processCertificateFromReadSeeker(packageFile, keepOption, reader)
	closeErr := reader.Close()
	if certErr != nil {
		return "", false, fmt.Errorf("certificate processing failed: %w", certErr)
	}
	if closeErr != nil {
		return "", false, fmt.Errorf("failed to close encrypted temporary APK: %w", closeErr)
	}
	if shouldSkip {
		return "", true, nil
	}

	reader, err = staged.Open()
	if err != nil {
		return "", false, fmt.Errorf("failed to reopen encrypted temporary APK: %w", err)
	}
	defer reader.Close()

	zipPath = reserveUniqueZipPath(zipPath, usedZipPaths)
	if err := acq.ZipWriter.CreateFileFromReader(zipPath, reader); err != nil {
		return "", false, fmt.Errorf("failed to add APK to archive: %w", err)
	}
	return zipPath, false, nil
}

func (p *Packages) processLargeAPKFromTemp(packageFile *adb.PackageFile, keepOption, zipPath string, acq *acquisition.Acquisition, usedZipPaths map[string]struct{}) (string, bool, error) {
	log.Debugf("APK %s exceeded streaming buffer limit; using temporary file for certificate check", packageFile.Path)

	tempPath, err := acq.StreamingPuller.PullToTempFile(packageFile.Path)
	if err != nil {
		return "", false, fmt.Errorf("failed to pull APK to temporary file: %w", err)
	}
	defer os.Remove(tempPath)

	shouldSkip, err := p.processCertificateFromPath(packageFile, keepOption, tempPath)
	if err != nil {
		return "", false, fmt.Errorf("certificate processing failed: %v", err)
	}
	if shouldSkip {
		return "", true, nil
	}

	zipPath = reserveUniqueZipPath(zipPath, usedZipPaths)
	if err := acq.ZipWriter.CreateFileFromPath(zipPath, tempPath); err != nil {
		return "", false, fmt.Errorf("failed to stream to archive: %v", err)
	}
	return zipPath, false, nil
}

func (p *Packages) processCertificateFromPath(packageFile *adb.PackageFile, keepOption, path string) (bool, error) {
	verified, cert, err := utils.VerifyCertificate(path)
	return p.applyCertificateResult(packageFile, keepOption, verified, cert, err)
}

func (p *Packages) processCertificateFromReadSeeker(packageFile *adb.PackageFile, keepOption string, reader io.ReadSeeker) (bool, error) {
	verified, cert, err := utils.VerifyCertificateFromReadSeeker(reader)
	return p.applyCertificateResult(packageFile, keepOption, verified, cert, err)
}

func (p *Packages) applyCertificateResult(packageFile *adb.PackageFile, keepOption string, verified bool, cert *apkverifier.CertInfo, err error) (bool, error) {
	if cert == nil {
		packageFile.CertificateError = "No certificate found"
		if err != nil {
			packageFile.CertificateError = err.Error()
		}
		packageFile.VerifiedCertificate = false
		return false, nil
	}

	// Set certificate information
	packageFile.Certificate = *cert
	packageFile.VerifiedCertificate = verified

	if err != nil {
		packageFile.CertificateError = err.Error()
	} else {
		packageFile.CertificateError = ""
	}

	// Only trust certificates from APKs whose signatures verified successfully.
	// A tampered APK may still expose the certificate used to sign the original.
	if verified && err == nil && utils.IsTrusted(*cert) {
		packageFile.TrustedCertificate = true
		if keepOption == apkRemoveTrusted {
			return true, nil // Skip this APK
		}
	}

	return false, nil
}

// processCertificate handles certificate verification and returns whether APK should be skipped
func (p *Packages) processCertificate(packageFile *adb.PackageFile, keepOption string, buffer *acquisition.StreamingBuffer) (bool, error) {
	return p.processCertificateFromReadSeeker(packageFile, keepOption, buffer.Reader())
}
