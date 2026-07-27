package utils

import (
	"testing"

	"github.com/avast/apkverifier"
)

func TestSignalCertificatesAreTrusted(t *testing.T) {
	for _, sha1 := range []string{
		"45989dc9ad8728c2aa9a82fa55503e34a8879374",
		"5c6740091301285db5409fdcd1b90f1ac3ba2dcf",
	} {
		t.Run(sha1, func(t *testing.T) {
			if !IsTrusted(apkverifier.CertInfo{Sha1: sha1}) {
				t.Errorf("Signal certificate %s is not trusted", sha1)
			}
		})
	}
}
