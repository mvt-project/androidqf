package adb

import "testing"

func TestParseFullFindOutputPreservesPathCharacters(t *testing.T) {
	out := "1700000000.25\t640\t42\tshell\tshell\t/sdcard/My File\nwith newline.txt\x00"
	files, err := parseFullFindOutput(out)
	if err != nil {
		t.Fatalf("parseFullFindOutput() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if files[0].Path != "/sdcard/My File\nwith newline.txt" {
		t.Fatalf("path = %q", files[0].Path)
	}
	if files[0].Size != 42 || files[0].Mode != "640" || files[0].ModifiedTime != 1700000000 {
		t.Fatalf("metadata = %+v", files[0])
	}
}

func TestParseFullFindOutputRejectsMalformedRecords(t *testing.T) {
	for _, out := range []string{
		"short record\x00",
		"invalid\t640\t42\tshell\tshell\t/path\x00",
		"1700000000\t640\tinvalid\tshell\tshell\t/path\x00",
	} {
		if _, err := parseFullFindOutput(out); err == nil {
			t.Fatalf("parseFullFindOutput(%q) error = nil", out)
		}
	}
}

func TestQuoteRemoteShellArgEscapesApostrophes(t *testing.T) {
	got := QuoteRemoteShellArg("/sdcard/user's files")
	want := `'/sdcard/user'"'"'s files'`
	if got != want {
		t.Fatalf("QuoteRemoteShellArg() = %q, want %q", got, want)
	}
}
