package modules

import (
	"errors"
	"testing"

	"github.com/avast/apkverifier"
	"github.com/mvt-project/androidqf/adb"
)

func TestShouldNotRemoveTrustedAPKWhenVerificationFails(t *testing.T) {
	packageFile := &adb.PackageFile{Path: "/data/app/com.example/base.apk"}
	cert := &apkverifier.CertInfo{
		Sha1: "38918a453d07199354f8b19af05ec6562ced5788",
	}
	verifyErr := errors.New("signature verification failed")

	if shouldRemoveTrustedAPK(packageFile, false, cert, verifyErr, apkRemoveTrusted) {
		t.Fatal("shouldRemoveTrustedAPK() = true, want false")
	}

	if packageFile.TrustedCertificate {
		t.Fatal("TrustedCertificate = true, want false")
	}
	if packageFile.VerifiedCertificate {
		t.Fatal("VerifiedCertificate = true, want false")
	}
	if packageFile.CertificateError != verifyErr.Error() {
		t.Fatalf("CertificateError = %q, want %q", packageFile.CertificateError, verifyErr.Error())
	}
}

func TestShouldRemoveTrustedAPKWhenVerified(t *testing.T) {
	packageFile := &adb.PackageFile{Path: "/data/app/com.example/base.apk"}
	cert := &apkverifier.CertInfo{
		Sha1: "38918a453d07199354f8b19af05ec6562ced5788",
	}

	if !shouldRemoveTrustedAPK(packageFile, true, cert, nil, apkRemoveTrusted) {
		t.Fatal("shouldRemoveTrustedAPK() = false, want true")
	}

	if !packageFile.TrustedCertificate {
		t.Fatal("TrustedCertificate = false, want true")
	}
	if !packageFile.VerifiedCertificate {
		t.Fatal("VerifiedCertificate = false, want true")
	}
	if packageFile.CertificateError != "" {
		t.Fatalf("CertificateError = %q, want empty", packageFile.CertificateError)
	}
}

func TestShouldRemoveTrustedAPKRespectsKeepAllSelection(t *testing.T) {
	packageFile := &adb.PackageFile{Path: "/data/app/com.example/base.apk"}
	cert := &apkverifier.CertInfo{
		Sha1: "38918a453d07199354f8b19af05ec6562ced5788",
	}

	if shouldRemoveTrustedAPK(packageFile, true, cert, nil, apkKeepAll) {
		t.Fatal("shouldRemoveTrustedAPK() = true, want false")
	}
}
