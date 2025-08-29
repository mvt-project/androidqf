// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"fmt"
	"io"
	"os"
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

func (p *Packages) getPathToLocalCopy(packageName, filePath string) string {
	fileName := ""
	if strings.Contains(filePath, "==/") {
		fileName = fmt.Sprintf(
			"_%s",
			strings.Replace(strings.Split(filePath, "==/")[1], ".apk", "", 1),
		)
	}

	localPath := filepath.Join(p.ApksPath, fmt.Sprintf("%s%s.apk", packageName, fileName))
	counter := 0
	for {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			break
		}

		counter++
		localPath = filepath.Join(
			p.ApksPath,
			fmt.Sprintf("%s%s_%d.apk", packageName, fileName, counter),
		)
	}

	return localPath
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

		// Ask if the user want to remove trusted packages
		log.Info("Would you like to remove copies of apps signed with a trusted certificate to limit the size of the output folder?")
		promptAll := promptui.Select{
			Label: "Remove",
			Items: []string{apkRemoveTrusted, apkKeepAll},
		}
		_, keepOption, err := promptAll.Run()
		if err != nil {
			return fmt.Errorf("failed to make selection for download option: %v",
				err)
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

					// Create zip path for APK
					fileName := ""
					if strings.Contains(packageFile.Path, "==/") {
						fileName = fmt.Sprintf("_%s", strings.Replace(strings.Split(packageFile.Path, "==/")[1], ".apk", "", 1))
					}
					zipPath := fmt.Sprintf("apks/%s%s.apk", packages[ip].Name, fileName)

					// Create certificate processor function
					var skipAPK bool
					certProcessor := func(reader io.Reader) error {
						// Verify certificate from stream
						verified, cert, err := utils.VerifyCertificateFromReader(reader)
						if cert == nil {
							packageFile.CertificateError = err.Error()
							packageFile.VerifiedCertificate = false
						} else {
							packageFile.Certificate = *cert
							packageFile.VerifiedCertificate = verified
							if err != nil {
								packageFile.CertificateError = err.Error()
							} else {
								packageFile.CertificateError = ""
								if utils.IsTrusted(*cert) {
									packageFile.TrustedCertificate = true
									if keepOption == apkRemoveTrusted {
										log.Debugf("Trusted APK skipped for streaming: %s", packageFile.Path)
										skipAPK = true
										return nil // Skip but don't error
									}
								}
							}
						}
						return nil
					}

					// Stream APK directly to encrypted zip with certificate processing
					err = acq.StreamAPKToZip(packageFile.Path, zipPath, certProcessor)
					if err != nil {
						packageFile.Error = fmt.Sprintf("Failed to stream to encrypted archive: %v", err)
						log.Debugf("ERROR: failed to stream %s to encrypted archive: %v", packageFile.Path, err)
						continue
					}

					// Skip if marked as trusted and removal requested
					if skipAPK {
						continue
					}

					log.Debugf("Streamed %s directly to encrypted archive as %s", packageFile.Path, zipPath)
				} else {
					// Traditional mode: download to local storage
					localPath := p.getPathToLocalCopy(packages[ip].Name, packageFile.Path)

					out, err := adb.Client.Pull(packageFile.Path, localPath)
					if err != nil {
						packageFile.Error = out
						log.Debugf("ERROR: failed to download %s: %s", packageFile.Path, out)
						continue
					}

					log.Debugf("Downloaded %s to %s", packageFile.Path, localPath)

					// Check the certificate
					verified, cert, err := utils.VerifyCertificate(localPath)
					if cert == nil {
						// Couldn't extract certificate
						log.Debugf("Couldn't parse certificate for app %s", localPath)
						packageFile.CertificateError = err.Error()
						packageFile.VerifiedCertificate = false
					} else {
						packageFile.Certificate = *cert
						packageFile.VerifiedCertificate = false
						if err != nil {
							// Extracted certificate but couldn't verify it
							packageFile.CertificateError = err.Error()
						} else {
							packageFile.CertificateError = ""
							packageFile.VerifiedCertificate = verified
							if utils.IsTrusted(*cert) {
								packageFile.TrustedCertificate = true
								if keepOption == apkRemoveTrusted {
									log.Debugf("Trusted APK removed: %s - %s",
										localPath, packageFile.SHA256)
									os.Remove(localPath)
								}
							}
						}
					}
				}
			}
		}
	}

	return saveDataToAcquisition(acq, "packages.json", &packages)
}
