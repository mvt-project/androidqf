// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
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

type Packages struct {
	StoragePath string
	ApksPath    string
}

func NewPackages() *Packages {
	return &Packages{}
}

func (p *Packages) Name() string {
	return "packages"
}

func (p *Packages) InitStorage(storagePath string) error {
	p.StoragePath = storagePath
	p.ApksPath = filepath.Join(storagePath, "apks")

	// Only create directory in traditional mode
	if storagePath != "" {
		err := os.Mkdir(p.ApksPath, 0o755)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to create apks folder: %v", err)
		}
	}

	return nil
}

func (p *Packages) getPathToLocalCopy(packageName, filePath string) (string, error) {
	suffix, err := p.extractFileName(filePath)
	if err != nil {
		return "", err
	}
	base := fmt.Sprintf("%s%s.apk", packageName, suffix)
	if !filepath.IsLocal(base) {
		return "", fmt.Errorf("non-local APK basename: %q", base)
	}
	localPath := filepath.Join(p.ApksPath, base)

	counter := 0
	for {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			break
		}
		counter++
		localPath = filepath.Join(
			p.ApksPath,
			fmt.Sprintf("%s%s_%d.apk", packageName, suffix, counter),
		)
	}
	return localPath, nil
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

		// Only ask about certificate removal for unencrypted output
		if acq.StreamingMode && acq.EncryptedWriter != nil {
			// For encrypted output, always keep all APKs (skip certificate checking)
			keepOption = apkKeepAll
		} else {
			// Ask if the user want to remove trusted packages for unencrypted output
			log.Info("Would you like to remove copies of apps signed with a trusted certificate to limit the size of the output folder?")
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

				if acq.StreamingMode && acq.EncryptedWriter != nil {
					// Streaming mode: stream directly to encrypted zip without temp files
					if err := p.processAPKStreaming(packages[ip].Name, packageFile, keepOption, acq); err != nil {
						log.Debugf("ERROR: failed to process APK %s: %v", packageFile.Path, err)
						continue
					}
				} else {
					// Traditional mode: download to local storage
					localPath, err := p.getPathToLocalCopy(packages[ip].Name, packageFile.Path)
					if err != nil {
						log.Errorf("Skipping APK with unsafe path %q: %v", packageFile.Path, err)
						packageFile.Error = err.Error()
						continue
					}

					out, err := adb.Client.Pull(packageFile.Path, localPath)
					if err != nil {
						packageFile.Error = out
						log.Debugf("ERROR: failed to download %s: %s", packageFile.Path, out)
						continue
					}

					log.Debugf("Downloaded %s to %s", packageFile.Path, localPath)

					// Check the certificate
					verified, cert, err := utils.VerifyCertificate(localPath)
					if shouldRemoveTrustedAPK(packageFile, verified, cert, err, keepOption) {
						log.Debugf("Trusted APK removed: %s - %s",
							localPath, packageFile.SHA256)
						if err := os.Remove(localPath); err != nil {
							log.Debugf("ERROR: failed to remove trusted APK %s: %v", localPath, err)
						}
					}
				}
			}
		}
	}

	return saveDataToAcquisition(acq, "packages.json", &packages)
}

func shouldRemoveTrustedAPK(packageFile *adb.PackageFile, verified bool, cert *apkverifier.CertInfo, certErr error, keepOption string) bool {
	if cert == nil {
		log.Debugf("Couldn't parse certificate for app %s", packageFile.Path)
		packageFile.CertificateError = "No certificate found"
		if certErr != nil {
			packageFile.CertificateError = certErr.Error()
		}
		packageFile.VerifiedCertificate = false
		return false
	}

	packageFile.Certificate = *cert
	packageFile.VerifiedCertificate = verified
	if certErr != nil {
		packageFile.CertificateError = certErr.Error()
	} else {
		packageFile.CertificateError = ""
	}

	if verified && utils.IsTrusted(*cert) {
		packageFile.TrustedCertificate = true
		return keepOption == apkRemoveTrusted
	}

	return false
}

// processAPKStreaming handles APK processing in streaming mode
func (p *Packages) processAPKStreaming(packageName string, packageFile *adb.PackageFile, keepOption string, acq *acquisition.Acquisition) error {
	zipPath, err := p.generateZipPath(packageName, packageFile.Path)
	if err != nil {
		log.Errorf("Skipping APK with unsafe path %q: %v", packageFile.Path, err)
		packageFile.Error = err.Error()
		return nil
	}

	// For encrypted output, skip certificate processing entirely
	if acq.EncryptedWriter != nil {
		log.Debugf("Skipping certificate check for encrypted output: %s", packageFile.Path)
	} else {
		// Process certificate and determine if APK should be skipped (unencrypted output only)
		shouldSkip, err := p.processCertificate(packageFile, keepOption, acq)
		if err != nil {
			packageFile.Error = fmt.Sprintf("Certificate processing failed: %v", err)
			return err
		}

		if shouldSkip {
			log.Debugf("Trusted APK skipped for streaming: %s", packageFile.Path)
			return nil
		}
	}

	// Stream APK directly to encrypted zip
	err = acq.StreamAPKToZip(packageFile.Path, zipPath, nil)
	if err != nil {
		packageFile.Error = fmt.Sprintf("Failed to stream to encrypted archive: %v", err)
		return err
	}

	log.Debugf("Streamed %s directly to encrypted archive as %s", packageFile.Path, zipPath)
	return nil
}

// processCertificate handles certificate verification and returns whether APK should be skipped
func (p *Packages) processCertificate(packageFile *adb.PackageFile, keepOption string, acq *acquisition.Acquisition) (bool, error) {
	// Pull APK to buffer for certificate verification
	buffer, err := acq.StreamingPuller.PullToBuffer(packageFile.Path)
	if err != nil {
		return false, fmt.Errorf("failed to pull APK for certificate verification: %v", err)
	}

	// Verify certificate from buffer using in-memory verification
	verified, cert, err := utils.VerifyCertificateFromReader(buffer.Reader())
	return shouldRemoveTrustedAPK(packageFile, verified, cert, err, keepOption), nil
}
