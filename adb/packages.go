// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/avast/apkverifier"
	"github.com/mvt-project/androidqf/log"
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
	Name       string        `json:"name"`
	Files      []PackageFile `json:"files"`
	Installer  string        `json:"installer"`
	UID        int           `json:"uid"`
	Disabled   bool          `json:"disabled"`
	System     bool          `json:"system"`
	ThirdParty bool          `json:"third_party"`
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

func (a *ADB) getPackageFiles(packageName string, fast bool) []PackageFile {
	out, err := a.Shell("pm", "path", QuoteRemoteShellArg(packageName))
	if err != nil {
		log.Errorf("Failed to get file paths for package %s: %v: %s", packageName, err, out)
		return []PackageFile{}
	}

	packageFiles := []PackageFile{}
	for _, line := range strings.Split(out, "\n") {
		packagePath := strings.TrimPrefix(strings.TrimSpace(line), "package:")
		if packagePath == "" {
			continue
		}

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

	return packageFiles
}

// GetPackages returns the list of installed package names.
func (a *ADB) GetPackages(fast bool) ([]Package, error) {
	attempts := []packageListAttempt{
		{args: []string{"pm", "list", "packages", "-U", "-u", "-i"}, withInstaller: true},
		{args: []string{"pm", "list", "packages", "-U", "-u"}, withInstaller: false},
		// Some older Samsung builds reject cross-user package access.
		{args: []string{"pm", "list", "packages", "-U", "-u", "-i", "--user", "0"}, withInstaller: true},
	}

	var (
		out     string
		attempt packageListAttempt
		err     error
	)
	for _, candidate := range attempts {
		out, err = a.Shell(candidate.args...)
		if err == nil {
			attempt = candidate
			break
		}
	}
	if err != nil {
		return []Package{}, fmt.Errorf("failed to launch `pm list packages` command: %v", err)
	}

	entries, err := parsePackageList(out, attempt.withInstaller)
	if err != nil {
		return []Package{}, err
	}
	packages := []Package{}
	for _, entry := range entries {
		newPackage := Package{
			Name:       entry.name,
			Installer:  entry.installer,
			UID:        entry.uid,
			Disabled:   false,
			System:     false,
			ThirdParty: false,
			Files:      a.getPackageFiles(entry.name, fast),
		}

		packages = append(packages, newPackage)
	}

	cmds := []map[string]string{
		{"field": "Disabled", "arg": "-d"},
		{"field": "System", "arg": "-s"},
		{"field": "ThirdParty", "arg": "-3"},
	}
	for _, cmd := range cmds {
		out, err = a.Shell("pm", "list", "packages", cmd["arg"])
		if err != nil && out == "" {
			log.Infof("Failed to get packages filtered by `%s`: %v: %s\n",
				cmd["arg"], err, out)
			continue
		}

		for _, line := range strings.Split(out, "\n") {
			packageName := strings.TrimPrefix(strings.TrimSpace(line), "package:")
			if packageName == "" {
				continue
			}

			for pIndex, p := range packages {
				if p.Name != packageName {
					continue
				}

				elems := reflect.ValueOf(&p).Elem()
				for i := 0; i < elems.NumField(); i++ {
					fieldName := elems.Type().Field(i).Name
					if fieldName == cmd["field"] {
						reflect.ValueOf(&packages[pIndex]).Elem().FieldByName(fieldName).SetBool(true)
					}
				}
			}
		}
	}

	return packages, nil
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
			log.Debugf("Ignoring unexpected package-list output: %s", line)
			continue
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
