// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"path/filepath"
	"strings"

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

func (p *Packages) Run(acq *acquisition.Acquisition, fast bool) error {
	log.Info("Collecting information on installed apps. This might take a while...")

	packages, err := adb.Client.GetPackages(fast)
	if err != nil {
		return fmt.Errorf("failed to retrieve list of installed packages: %v", err)
	}

	log.Infof(
		"Found a total of %d installed packages",
		len(packages),
	)

	log.Info("Would you like to download copies of all apps or only non-system ones?")
	downloadPrompt := promptui.Select{
		Label: "Download",
		Items: []string{apkAll, apkNotSystem, apkNone},
	}
	_, download, err := downloadPrompt.Run()
	if err != nil {
		return fmt.Errorf("failed to make selection for download option: %v", err)
	}

	// If the user decides to not download any APK, then we skip this.
	// Otherwise we walk through the list of package, pull the files, and hash them.
	if download != apkNone {

		var keepOption string

		if acq.ZipWriter != nil && acq.ZipWriter.IsEncrypted() {
			keepOption = apkKeepAll
		} else {
			log.Info("Would you like to remove copies of apps signed with a trusted certificate to limit the size of the output archive?")
			promptAll := promptui.Select{
				Label: "Remove",
				Items: []string{apkRemoveTrusted, apkKeepAll},
			}
			_, keepOption, err = promptAll.Run()
			if err != nil {
				return fmt.Errorf("failed to make selection for download option: %v",
					err)
			}
		}

		for ip := 0; ip < len(packages); ip++ {
			// If we the user did not request to download all packages and if
			// the package is marked as system, we skip it.
			if download != apkAll && packages[ip].System {
				continue
			}

			log.Debugf("Found Android package: %s", packages[ip].Name)

			for ipf := 0; ipf < len(packages[ip].Files); ipf++ {
				packageFile := &packages[ip].Files[ipf]

				if err := p.processAPKStreaming(packages[ip].Name, packageFile, keepOption, acq); err != nil {
					log.Debugf("ERROR: failed to process APK %s: %v", packageFile.Path, err)
					continue
				}
			}
		}
	}

	return saveDataToAcquisition(acq, "packages.json", &packages)
}

func (p *Packages) processAPKStreaming(packageName string, packageFile *adb.PackageFile, keepOption string, acq *acquisition.Acquisition) error {
	zipPath, err := p.generateZipPath(packageName, packageFile.Path)
	if err != nil {
		log.Errorf("Skipping APK with unsafe path %q: %v", packageFile.Path, err)
		packageFile.Error = err.Error()
		return nil
	}

	if acq.ZipWriter != nil && acq.ZipWriter.IsEncrypted() {
		log.Debugf("Skipping certificate check for encrypted archive: %s", packageFile.Path)
		err = acq.StreamAPKToZip(packageFile.Path, zipPath, nil)
		if err != nil {
			packageFile.Error = fmt.Sprintf("Failed to stream to archive: %v", err)
			return err
		}
	} else {
		buffer, err := acq.StreamingPuller.PullToBuffer(packageFile.Path)
		if err != nil {
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
		err = acq.ZipWriter.CreateFileFromReader(zipPath, buffer.Reader())
		if err != nil {
			packageFile.Error = fmt.Sprintf("Failed to stream to archive: %v", err)
			return err
		}
	}

	log.Debugf("Streamed %s directly to archive as %s", packageFile.Path, zipPath)
	return nil
}

// processCertificate handles certificate verification and returns whether APK should be skipped
func (p *Packages) processCertificate(packageFile *adb.PackageFile, keepOption string, buffer *acquisition.StreamingBuffer) (bool, error) {
	verified, cert, err := utils.VerifyCertificateFromReader(buffer.Reader())
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

	// Check if certificate is trusted and should be removed
	if utils.IsTrusted(*cert) {
		packageFile.TrustedCertificate = true
		if keepOption == apkRemoveTrusted {
			return true, nil // Skip this APK
		}
	}

	return false, nil
}
