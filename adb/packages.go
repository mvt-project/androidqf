// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/avast/apkverifier"
)

type PackageFile struct {
	Path                string               `json:"path"`
	LocalName           string               `json:"local_name"`
	MD5                 string               `json:"md5"`
	SHA1                string               `json:"sha1"`
	SHA256              string               `json:"sha256"`
	SHA512              string               `json:"sha512"`
	Error               string               `json:"error"`
	VerifiedCertificate bool                 `json:"verified_certificate"`
	Certificate         apkverifier.CertInfo `json:"certificate"`
	CertificateError    string               `json:"certificate_error"`
	TrustedCertificate  bool                 `json:"trusted_certificate"`
}

type Package struct {
	UserID     int           `json:"user_id"`
	Installed  *bool         `json:"installed,omitempty"`
	Name       string        `json:"name"`
	Files      []PackageFile `json:"files"`
	Installer  string        `json:"installer"`
	UID        int           `json:"uid"`
	Disabled   bool          `json:"disabled"`
	System     bool          `json:"system"`
	ThirdParty bool          `json:"third_party"`
	FilesError string        `json:"files_error,omitempty"`
}

type packageListAttempt struct {
	args          []string
	withInstaller bool
}

type packageListEntry struct {
	name      string
	installer string
	uid       int
}

func (a *ADB) getPackageFiles(packageName string, userID int, fast bool) ([]PackageFile, error) {
	out, err := a.Shell("pm", "path", "--user", strconv.Itoa(userID), QuoteRemoteShellArg(packageName))
	if err != nil {
		return []PackageFile{}, fmt.Errorf("failed to get file paths: %w: %s", err, out)
	}

	packageFiles := []PackageFile{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "package:/") {
			return []PackageFile{}, fmt.Errorf("unrecognized APK path output %q", line)
		}
		packagePath := strings.TrimPrefix(line, "package:")

		packageFile := PackageFile{
			Path: packagePath,
		}

		if !fast {
			// Not sure if this is useful or not considering packages may
			// be downloaded later on
			quotedPackagePath := QuoteRemoteShellArg(packagePath)
			md5Out, err := a.Shell("md5sum", quotedPackagePath)
			if err == nil {
				packageFile.MD5 = strings.SplitN(md5Out, " ", 2)[0]
			}
			sha1Out, err := a.Shell("sha1sum", quotedPackagePath)
			if err == nil {
				packageFile.SHA1 = strings.SplitN(sha1Out, " ", 2)[0]
			}
			sha256Out, err := a.Shell("sha256sum", quotedPackagePath)
			if err == nil {
				packageFile.SHA256 = strings.SplitN(sha256Out, " ", 2)[0]
			}
			sha512Out, err := a.Shell("sha512sum", quotedPackagePath)
			if err == nil {
				packageFile.SHA512 = strings.SplitN(sha512Out, " ", 2)[0]
			}
		}

		packageFiles = append(packageFiles, packageFile)
	}
	if len(packageFiles) == 0 {
		return packageFiles, fmt.Errorf("no APK paths returned for user %d", userID)
	}

	return packageFiles, nil
}

// GetPackages returns per-user package records, including retained uninstalled
// records. A non-nil error can accompany successfully collected users.
func (a *ADB) GetPackages(fast bool) ([]Package, error) {
	packages, _, err := a.GetPackagesWithUsers(fast)
	return packages, err
}

func parsePackageList(out string, withInstaller bool) ([]packageListEntry, error) {
	var entries []packageListEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "package:") {
			return nil, fmt.Errorf("unrecognized package-list output %q", line)
		}

		expectedFields := 2
		uidIndex := 1
		if withInstaller {
			expectedFields = 3
			uidIndex = 2
		}
		if len(fields) < expectedFields {
			return nil, fmt.Errorf("malformed package-list output %q", line)
		}

		entry := packageListEntry{name: strings.TrimPrefix(fields[0], "package:")}
		if entry.name == "" {
			return nil, fmt.Errorf("malformed package-list output %q", line)
		}
		if withInstaller {
			if !strings.HasPrefix(fields[1], "installer=") {
				return nil, fmt.Errorf("malformed installer field in %q", line)
			}
			entry.installer = strings.TrimPrefix(fields[1], "installer=")
		}
		if !strings.HasPrefix(fields[uidIndex], "uid:") {
			return nil, fmt.Errorf("malformed UID field in %q", line)
		}
		uid, err := strconv.Atoi(strings.TrimPrefix(fields[uidIndex], "uid:"))
		if err != nil {
			return nil, fmt.Errorf("malformed UID field in %q: %w", line, err)
		}
		entry.uid = uid
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("package-list output contained no package records")
	}
	return entries, nil
}

// GetPackagePaths returns a list of file paths associated with the provided
// package name.
func (a *ADB) GetPackagePaths(packageName string) ([]string, error) {
	out, err := a.Shell("pm", "path", QuoteRemoteShellArg(packageName))
	if err != nil {
		return []string{}, fmt.Errorf("failed to launch `pm path` command: %v",
			err)
	}

	packagePaths := []string{}
	for _, line := range strings.Split(out, "\n") {
		packagePath := strings.TrimPrefix(strings.TrimSpace(line), "package:")
		if packagePath == "" {
			continue
		}

		packagePaths = append(packagePaths, packagePath)
	}

	return packagePaths, nil
}
