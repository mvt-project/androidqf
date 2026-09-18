package adb

import "testing"

func TestParsePackageListWithInstaller(t *testing.T) {
	out := "package:org.example installer=com.android.vending uid:10123\n"
	entries, err := parsePackageList(out, true)
	if err != nil {
		t.Fatalf("parsePackageList() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if entries[0].name != "org.example" || entries[0].installer != "com.android.vending" || entries[0].uid != 10123 {
		t.Fatalf("entry = %+v", entries[0])
	}
}

func TestParsePackageListWithoutInstaller(t *testing.T) {
	entries, err := parsePackageList("package:org.example uid:10123", false)
	if err != nil {
		t.Fatalf("parsePackageList() error = %v", err)
	}
	if len(entries) != 1 || entries[0].installer != "" || entries[0].uid != 10123 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestParsePackageListRejectsEmptyOutput(t *testing.T) {
	if _, err := parsePackageList("", true); err == nil {
		t.Fatal("parsePackageList() error = nil")
	}
}

func TestParsePackageListRejectsMalformedPackageRecords(t *testing.T) {
	for _, line := range []string{
		"package:org.example",
		"package:org.example unexpected uid:10123",
		"package:org.example installer=com.android.vending uid:not-a-number",
	} {
		if _, err := parsePackageList(line, true); err == nil {
			t.Fatalf("parsePackageList(%q) error = nil", line)
		}
	}
}
