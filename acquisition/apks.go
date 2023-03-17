// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/botherder/go-savetime/hashes"
	"github.com/manifoldco/promptui"
	"github.com/mvt/androidqf/adb"
	"github.com/mvt/androidqf/log"
	"github.com/mvt/androidqf/utils"
)

const (
	apkAll           = "All"
	apkNotSystem     = "Only non-system packages"
	apkNone          = "Do not download any"
	apkRemoveTrusted = "Yes"
	apkKeepAll       = "No"
)

func (a *Acquisition) getPathToLocalCopy(packageName, filePath string) string {
	fileName := ""
	if strings.Contains(filePath, "==/") {
		fileName = fmt.Sprintf("_%s", strings.Replace(strings.Split(filePath, "==/")[1], ".apk", "", 1))
	}

	localPath := filepath.Join(a.APKSPath, fmt.Sprintf("%s%s.apk", packageName, fileName))
	counter := 0
	for {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			break
		}

		counter++
		localPath = filepath.Join(a.APKSPath, fmt.Sprintf("%s%s_%d.apk", packageName, fileName, counter))
	}

	return localPath
}

func (a *Acquisition) savePackageJson(packages []adb.Package) error {
	packagesJSONPath := filepath.Join(a.StoragePath, "packages.json")
	packagesJSON, err := os.Create(packagesJSONPath)
	if err != nil {
		return fmt.Errorf("failed to save list of installed packages to file: %v",
			err)
	}
	defer packagesJSON.Close()

	buf, _ := json.MarshalIndent(packages, "", "    ")

	packagesJSON.WriteString(string(buf[:]))
	packagesJSON.Sync()

	return nil
}

func (a *Acquisition) DownloadAPKs() error {
	var downloadOption string
	var keepOption string
	var err error
	log.Info("Downloading copies of installed apps. This might take a while...")

	packages, err := a.ADB.GetPackages()
	if err != nil {
		return fmt.Errorf("failed to retrieve list of installed packages: %v", err)
	}

	log.Debugf("Found a total of %d installed packages", len(packages))

	fmt.Println("Would you like to download copies of all apps or only non-system ones?")
	promptAll := promptui.Select{
		Label: "Download",
		Items: []string{apkAll, apkNotSystem, apkNone},
	}
	_, downloadOption, err = promptAll.Run()
	if err != nil {
		return fmt.Errorf("failed to make selection for download option: %v",
			err)
	}
	if downloadOption == apkNone {
		log.Debug("No APK download was chosen by the user, only saving package information")
		return a.savePackageJson(packages)
	}

	fmt.Println("Would you like to remove copies of apps signed with a trusted certificate to limit the size of the output folder?")
	promptAll = promptui.Select{
		Label: "Remove",
		Items: []string{apkRemoveTrusted, apkKeepAll},
	}
	_, keepOption, err = promptAll.Run()
	if err != nil {
		return fmt.Errorf("failed to make selection for download option: %v",
			err)
	}

	// Otherwise we walk through the list of package, pull the files, and hash them.
	for i, p := range packages {
		log.Debugf("Found Android package: %s", p.Name)

		pFilePaths, err := a.ADB.GetPackagePaths(p.Name)
		if err != nil {
			continue
		}
		for _, pFilePath := range pFilePaths {
			localPath := a.getPathToLocalCopy(p.Name, pFilePath)

			out, err := a.ADB.Pull(pFilePath, localPath)
			if err != nil {
				file := adb.PackageFile{
					Path:      pFilePath,
					LocalName: "",
					SHA256:    "",
					SHA1:      "",
					MD5:       "",
					Error:     out,
				}
				packages[i].Files = append(packages[i].Files, file)
				continue
			}

			log.Debugf("Downloaded %s to %s", pFilePath, localPath)
			sha256, _ := hashes.FileSHA256(localPath)
			sha1, _ := hashes.FileSHA1(localPath)
			md5, _ := hashes.FileMD5(localPath)
			verified, cert, err := utils.VerifyCertificate(localPath)
			var file adb.PackageFile
			file.Path = pFilePath
			file.LocalName = filepath.Base(localPath)
			file.SHA256 = sha256
			file.SHA1 = sha1
			file.MD5 = md5
			if err != nil {
				file.CertificateError = err.Error()
				file.VerifiedCertificate = false
			} else {
				file.CertificateError = ""
				file.Certificate = *cert
				file.VerifiedCertificate = verified
				if utils.IsTrusted(*cert) {
					file.TrustedCertificate = true
					// Remove the APK
					log.Debugf("Trusted APK removed: %s - %s",
						file.LocalName, file.SHA256)
					if keepOption == apkRemoveTrusted {
						os.Remove(localPath)
					}
				} else {
					// remove system apps if asked for it
					if (p.System) && (downloadOption != apkNotSystem) {
						os.Remove(localPath)
					}
					file.TrustedCertificate = false
				}
			}

			packages[i].Files = append(packages[i].Files, file)
		}
	}

	// Store the results into a JSON file.
	return a.savePackageJson(packages)
}
