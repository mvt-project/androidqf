package modules

import (
	"errors"
	"testing"

	"github.com/avast/apkverifier"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/utils"
)

func TestReserveUniqueZipPathAddsCounterForDuplicateAPKs(t *testing.T) {
	used := make(map[string]struct{})
	paths := []string{
		"apks/com.example.apk",
		"apks/com.example.apk",
		"apks/com.example.apk",
	}
	want := []string{
		"apks/com.example.apk",
		"apks/com.example_1.apk",
		"apks/com.example_2.apk",
	}

	for i, path := range paths {
		if got := reserveUniqueZipPath(path, used); got != want[i] {
			t.Fatalf("reserveUniqueZipPath(%q) = %q, want %q", path, got, want[i])
		}
	}
}

func TestGenerateZipPathWithReservationKeepsSplitAPKNamesUnique(t *testing.T) {
	packages := NewPackages()
	used := make(map[string]struct{})
	files := []string{
		"/data/app/com.example-1/base.apk",
		"/data/app/com.example-1/split_config.en.apk",
		"/data/app/~~abc==/base.apk",
		"/data/app/~~abc==/split_config.en.apk",
	}
	want := []string{
		"apks/com.example.apk",
		"apks/com.example_1.apk",
		"apks/com.example_base.apk",
		"apks/com.example_split_config.en.apk",
	}

	for i, file := range files {
		zipPath, err := packages.generateZipPath("com.example", file)
		if err != nil {
			t.Fatalf("generateZipPath(%q) error = %v", file, err)
		}
		if got := reserveUniqueZipPath(zipPath, used); got != want[i] {
			t.Fatalf("reserved zip path for %q = %q, want %q", file, got, want[i])
		}
	}
}

func TestApplyCertificateResultHonorsTrustedCertificateRemoval(t *testing.T) {
	trustedCertificates := utils.ValidCertificates()
	if len(trustedCertificates) == 0 {
		t.Fatal("ValidCertificates() returned no trusted certificates")
	}

	packageFile := &adb.PackageFile{}
	cert := &apkverifier.CertInfo{Sha1: trustedCertificates[0]}
	skipped, err := NewPackages().applyCertificateResult(packageFile, apkRemoveTrusted, true, cert, nil)
	if err != nil {
		t.Fatalf("applyCertificateResult() error = %v", err)
	}
	if !skipped {
		t.Fatal("applyCertificateResult() did not skip a trusted certificate")
	}
	if !packageFile.TrustedCertificate || !packageFile.VerifiedCertificate {
		t.Fatalf("certificate flags = trusted:%v verified:%v, want both true", packageFile.TrustedCertificate, packageFile.VerifiedCertificate)
	}
	if packageFile.Certificate.Sha1 != cert.Sha1 {
		t.Fatalf("stored certificate SHA-1 = %q, want %q", packageFile.Certificate.Sha1, cert.Sha1)
	}
}

func TestApplyCertificateResultKeepsUnverifiedTrustedAPK(t *testing.T) {
	trustedCertificates := utils.ValidCertificates()
	if len(trustedCertificates) == 0 {
		t.Fatal("ValidCertificates() returned no trusted certificates")
	}

	verificationErr := errors.New("APK signature verification failed")
	packageFile := &adb.PackageFile{}
	cert := &apkverifier.CertInfo{Sha1: trustedCertificates[0]}
	skipped, err := NewPackages().applyCertificateResult(packageFile, apkRemoveTrusted, false, cert, verificationErr)
	if err != nil {
		t.Fatalf("applyCertificateResult() error = %v", err)
	}
	if skipped {
		t.Fatal("applyCertificateResult() skipped an APK whose signature did not verify")
	}
	if packageFile.TrustedCertificate {
		t.Fatal("unverified certificate was marked trusted")
	}
	if packageFile.VerifiedCertificate {
		t.Fatal("failed signature was marked verified")
	}
	if packageFile.CertificateError != verificationErr.Error() {
		t.Fatalf("certificate error = %q, want %q", packageFile.CertificateError, verificationErr)
	}
}
