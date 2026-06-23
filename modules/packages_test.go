package modules

import "testing"

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
