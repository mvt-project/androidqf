package cmd

import "testing"

func TestParseStatHandlesProcessNamesAndFields(t *testing.T) {
	process := ProcessInfo{}
	stat := "42 (worker name) S 1 2 3 4 5 6 7 8 9 10 1200 340 14 15 -5 0"

	if err := process.parseStat(stat); err != nil {
		t.Fatalf("parseStat() error = %v", err)
	}

	if process.Pid != 42 || process.Filename != "worker name" || process.State != "S" {
		t.Fatalf("identity fields = %+v", process)
	}
	if process.Ppid != 1 || process.Pgroup != 2 || process.Psid != 3 {
		t.Fatalf("relationship fields = %+v", process)
	}
	if process.UserTime != 1200 || process.KernelTime != 340 || process.Priority != -5 {
		t.Fatalf("accounting fields = %+v", process)
	}
}

func TestParseStatUsesLastClosingParenthesis(t *testing.T) {
	process := ProcessInfo{}
	stat := "7 (worker) name) R 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17"

	if err := process.parseStat(stat); err != nil {
		t.Fatalf("parseStat() error = %v", err)
	}
	if process.Filename != "worker) name" {
		t.Fatalf("filename = %q, want %q", process.Filename, "worker) name")
	}
}

func TestParseStatRejectsMalformedInput(t *testing.T) {
	for _, stat := range []string{"", "1 worker", "1 (worker) S 1"} {
		process := ProcessInfo{}
		if err := process.parseStat(stat); err == nil {
			t.Fatalf("parseStat(%q) error = nil", stat)
		}
	}
}
