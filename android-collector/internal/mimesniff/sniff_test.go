package mimesniff

import (
	"bytes"
	"net/http"
	"testing"
)

// Compare with the standard library to preserve the collector's fallback MIME
// detection, including whitespace, binary data, and the 512-byte sniff limit.
func TestDetectContentTypeMatchesHTTP(t *testing.T) {
	samples := [][]byte{
		nil,
		[]byte("plain forensic text"),
		[]byte("\t\n <!DOCTYPE html><html>"),
		[]byte("<?xml version=\"1.0\"?>"),
		[]byte("%PDF-1.7"),
		[]byte("\x89PNG\r\n\x1a\n"),
		[]byte("GIF89a"),
		[]byte("\xff\xd8\xff"),
		[]byte("PK\x03\x04"),
		[]byte("\x00\x01\x02\x03"),
		append(bytes.Repeat([]byte(" "), 511), []byte("<html>")...),
		append(bytes.Repeat([]byte("a"), 512), 0),
	}
	for _, sample := range samples {
		got := DetectContentType(sample)
		want := http.DetectContentType(sample)
		if got != want {
			t.Errorf("DetectContentType(%q) = %q, want %q", sample, got, want)
		}
	}
}

func FuzzDetectContentTypeMatchesHTTP(f *testing.F) {
	f.Add([]byte("forensic evidence"))
	f.Add([]byte("\x00\xff\x01"))
	f.Add([]byte("\n<html>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if got, want := DetectContentType(data), http.DetectContentType(data); got != want {
			t.Fatalf("DetectContentType(%q) = %q, want %q", data, got, want)
		}
	})
}
