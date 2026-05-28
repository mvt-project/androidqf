// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package adb

import "testing"

func TestParsePlatformToolsVersion(t *testing.T) {
	output := `Android Debug Bridge version 1.0.41
Version 36.0.2-14143358
Installed as /usr/bin/adb
Running on Linux 6.18.15-1.qubes.fc41.x86_64 (x86_64)
`

	version, err := parsePlatformToolsVersion(output)
	if err != nil {
		t.Fatalf("parsePlatformToolsVersion returned error: %v", err)
	}

	expected := platformToolsVersion{major: 36, minor: 0, patch: 2}
	if version != expected {
		t.Fatalf("expected %s, got %s", expected, version)
	}
}

func TestParsePlatformToolsVersionRejectsMalformedOutput(t *testing.T) {
	_, err := parsePlatformToolsVersion("Android Debug Bridge version 1.0.41")
	if err == nil {
		t.Fatal("expected parsePlatformToolsVersion to reject output without a platform-tools version")
	}
}

func TestPlatformToolsVersionMinimum(t *testing.T) {
	tests := []struct {
		name      string
		version   platformToolsVersion
		supported bool
	}{
		{
			name:      "previous one year cutoff release",
			version:   platformToolsVersion{major: 36, minor: 0, patch: 0},
			supported: false,
		},
		{
			name:      "minimum supported release",
			version:   platformToolsVersion{major: 36, minor: 0, patch: 2},
			supported: true,
		},
		{
			name:      "newer release",
			version:   platformToolsVersion{major: 37, minor: 0, patch: 0},
			supported: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.version.isAtLeast(minimumPlatformToolsVersion); got != test.supported {
				t.Fatalf("expected supported=%v for %s, got %v", test.supported, test.version, got)
			}
		})
	}
}
