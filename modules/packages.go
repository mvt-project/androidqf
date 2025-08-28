// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this source code is governed by the MVT License 1.1
// which can be found in the LICENSE file.

package modules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/log"
	"github.com/mvt-project/androidqf/utils"
	"github.com/spf13/afero"
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
	return nil
}

func (p *Packages) getPathToLocalCopy(fs afero.Fs, packageName, filePath string) string {
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
		if _, err := fs.Stat(localPath); os.IsNotExist(err) {
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

func (p *Packages) downloadAndProcessAPK(acq *acquisition.Acquisition, packageFile *adb.PackageFile, localPath string, keepOption string) error {
	if acq.UseMemoryFs {
		// For memory filesystem, pull APK data directly to memory buffer
		var apkBuffer bytes.Buffer

		// Pull APK content directly to memory using shell cat
		err := adb.Client.PullToWriter(packageFile.Path, &apkBuffer)
		if err != nil {
			packageFile.Error = fmt.Sprintf("failed to pull APK to memory: %v", err)
			log.Debugf("ERROR: failed to download %s: %v", packageFile.Path, err)
			return fmt.Errorf("failed to pull APK: %v", err)
		}

		// For certificate verification, we need a temporary file (verification only)
		// This is the only disk touch and is immediately cleaned up
		tempFile, err := os.CreateTemp("", "androidqf_cert_verify_*")
		if err != nil {
			log.Debugf("Failed to create temp file for certificate verification: %v", err)
			// Skip certificate verification if we can't create temp file
			packageFile.CertificateError = "Could not verify certificate: temp file creation failed"
			packageFile.VerifiedCertificate = false
		} else {
			tempPath := tempFile.Name()

			// Write APK data to temp file for certificate verification only
			_, err = tempFile.Write(apkBuffer.Bytes())
			tempFile.Close()

			if err != nil {
				os.Remove(tempPath)
				log.Debugf("Failed to write temp file for certificate verification: %v", err)
				packageFile.CertificateError = "Could not verify certificate: temp file write failed"
				packageFile.VerifiedCertificate = false
			} else {
				// Verify certificate using the temporary file (cleanup immediately after)
				verified, cert, err := utils.VerifyCertificate(tempPath)
				os.Remove(tempPath) // Immediate cleanup

				if cert == nil {
					log.Debugf("Couldn't parse certificate for app %s", localPath)
					packageFile.CertificateError = err.Error()
					packageFile.VerifiedCertificate = false
				} else {
					packageFile.Certificate = *cert
					packageFile.VerifiedCertificate = false
					if err != nil {
						packageFile.CertificateError = err.Error()
					} else {
						packageFile.CertificateError = ""
						packageFile.VerifiedCertificate = verified
						if utils.IsTrusted(*cert) {
							packageFile.TrustedCertificate = true
							if keepOption == apkRemoveTrusted {
								log.Debugf("Trusted APK not stored in memory: %s - %s",
									localPath, packageFile.SHA256)
								return nil // Don't store in memory filesystem
							}
						}
					}
				}
			}
		}

		// Ensure the apks directory exists in memory filesystem
		err = acq.Fs.MkdirAll(p.ApksPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create apks directory in memory: %v", err)
		}

		// Write APK data directly from memory buffer to memory filesystem
		memFile, err := acq.Fs.Create(localPath)
		if err != nil {
			return fmt.Errorf("failed to create APK file in memory: %v", err)
		}
		defer memFile.Close()

		_, err = memFile.Write(apkBuffer.Bytes())
		if err != nil {
			return fmt.Errorf("failed to write APK to memory filesystem: %v", err)
		}

		log.Debugf("Downloaded %s directly to memory filesystem at %s (%d bytes)", packageFile.Path, localPath, apkBuffer.Len())

	} else {
		// For disk filesystem, use traditional approach
		// Ensure the apks directory exists
		err := acq.Fs.MkdirAll(p.ApksPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create apks directory: %v", err)
		}

		out, err := adb.Client.Pull(packageFile.Path, localPath)
		if err != nil {
			packageFile.Error = out
			log.Debugf("ERROR: failed to download %s: %s", packageFile.Path, out)
			return fmt.Errorf("failed to pull APK: %s", out)
		}

		log.Debugf("Downloaded %s to %s", packageFile.Path, localPath)

		// Check the certificate
		verified, cert, err := utils.VerifyCertificate(localPath)
		if cert == nil {
			log.Debugf("Couldn't parse certificate for app %s", localPath)
			packageFile.CertificateError = err.Error()
			packageFile.VerifiedCertificate = false
		} else {
			packageFile.Certificate = *cert
			packageFile.VerifiedCertificate = false
			if err != nil {
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

	return nil
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

		if acq.UseMemoryFs {
			log.Info("Using in-memory processing for APK files - APK data pulled directly to memory")
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
				localPath := p.getPathToLocalCopy(acq.Fs, packages[ip].Name, packageFile.Path)

				err := p.downloadAndProcessAPK(acq, packageFile, localPath, keepOption)
				if err != nil {
					log.Debugf("Failed to process APK %s: %v", packageFile.Path, err)
					continue
				}
			}
		}
	}

	return saveCommandOutputJson(acq.Fs, filepath.Join(p.StoragePath, "packages.json"), &packages)
}
