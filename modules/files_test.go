package modules

import (
	"testing"

	"github.com/mvt-project/androidqf/adb"
)

type recordingFileFinder struct {
	findCalls     []string
	findHashCalls []string
}

func (f *recordingFileFinder) Find(path string) ([]adb.FileInfo, error) {
	f.findCalls = append(f.findCalls, path)
	return nil, nil
}

func (f *recordingFileFinder) FindHash(path string) ([]adb.FileInfo, error) {
	f.findHashCalls = append(f.findHashCalls, path)
	return nil, nil
}

func TestFindFilesUsesRequestedCollectorMode(t *testing.T) {
	finder := &recordingFileFinder{}

	if _, err := findFiles(finder, "/without-hashes", false); err != nil {
		t.Fatalf("find without hashes: %v", err)
	}
	if _, err := findFiles(finder, "/with-hashes", true); err != nil {
		t.Fatalf("find with hashes: %v", err)
	}

	if len(finder.findCalls) != 1 || finder.findCalls[0] != "/without-hashes" {
		t.Fatalf("Find calls = %v, want [/without-hashes]", finder.findCalls)
	}
	if len(finder.findHashCalls) != 1 || finder.findHashCalls[0] != "/with-hashes" {
		t.Fatalf("FindHash calls = %v, want [/with-hashes]", finder.findHashCalls)
	}
}
