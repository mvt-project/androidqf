// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// minimumPlatformToolsVersion is the oldest Android SDK Platform-Tools release
// within the supported one-year window as of 2026-05-28.
var minimumPlatformToolsVersion = platformToolsVersion{major: 36, minor: 0, patch: 2}

var platformToolsVersionRE = regexp.MustCompile(`(?m)^Version\s+([0-9]+)\.([0-9]+)\.([0-9]+)(?:[-\s]|$)`)

type platformToolsVersion struct {
	major int
	minor int
	patch int
}

func validatePlatformToolsVersion(adbPath string) error {
	out, err := exec.Command(adbPath, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check adb platform-tools version: %v: %s", err, strings.TrimSpace(string(out)))
	}

	version, err := parsePlatformToolsVersion(string(out))
	if err != nil {
		return err
	}
	if !version.isAtLeast(minimumPlatformToolsVersion) {
		return fmt.Errorf("adb platform-tools %s is too old; need %s or newer", version, minimumPlatformToolsVersion)
	}

	return nil
}

func parsePlatformToolsVersion(output string) (platformToolsVersion, error) {
	match := platformToolsVersionRE.FindStringSubmatch(output)
	if match == nil {
		return platformToolsVersion{}, fmt.Errorf("failed to parse adb platform-tools version from adb --version output")
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])

	return platformToolsVersion{major: major, minor: minor, patch: patch}, nil
}

func (v platformToolsVersion) isAtLeast(minimum platformToolsVersion) bool {
	if v.major != minimum.major {
		return v.major > minimum.major
	}
	if v.minor != minimum.minor {
		return v.minor > minimum.minor
	}
	return v.patch >= minimum.patch
}

func (v platformToolsVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}
