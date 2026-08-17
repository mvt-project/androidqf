package main

import (
	"strings"
	"testing"

	"github.com/mvt-project/androidqf/modules"
)

func TestBuildOptionsNoFlagsKeepsInteractiveDefaults(t *testing.T) {
	opts, err := buildOptions(false, false, "", "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *opts != (modules.Options{}) {
		t.Fatalf("opts = %+v, want zero value", *opts)
	}
}

func TestBuildOptionsInvalidValueFails(t *testing.T) {
	tests := []struct {
		name                                                                      string
		backup, download, removeTrusted, intrusionLogs, hashFiles, browserHistory string
		wantErr                                                                   string
	}{
		{"backup", "maybe", "", "", "", "", "", "invalid -backup value"},
		{"download", "", "some", "", "", "", "", "invalid -download value"},
		{"remove-trusted", "", "", "nope", "", "", "", "invalid -remove-trusted value"},
		{"intrusion-logs", "", "", "", "never", "", "", "invalid -intrusion-logs value"},
		{"hash-files", "", "", "", "", "maybe", "", "invalid -hash-files value"},
		{"browser-history", "", "", "", "", "", "maybe", "invalid -browser-history value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildOptions(false, false, tt.backup, tt.download, tt.removeTrusted, tt.intrusionLogs, tt.hashFiles, tt.browserHistory, "")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuildOptionsNonInteractiveUnknownModuleFails(t *testing.T) {
	_, err := buildOptions(false, true, "", "", "", "", "", "", "typo")
	if err == nil || !strings.Contains(err.Error(), "unknown -module value") {
		t.Fatalf("err = %v, want unknown -module error", err)
	}
}

func TestBuildOptionsNonInteractiveMissingFlagsFails(t *testing.T) {
	_, err := buildOptions(false, true, "", "", "", "", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "-non-interactive requires") {
		t.Fatalf("err = %v, want missing flags error", err)
	}
}

func TestBuildOptionsNonInteractiveModuleFilter(t *testing.T) {
	_, err := buildOptions(false, true, "none", "", "", "", "", "", "backup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildOptionsFullInvocation(t *testing.T) {
	opts, err := buildOptions(true, true, "sms", "all", "no", "no", "yes", "yes", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Fast || !opts.NonInteractive {
		t.Fatalf("opts = %+v, want Fast and NonInteractive set", *opts)
	}
	for _, check := range []struct {
		got   string
		parse func(string) (string, error)
		token string
	}{
		{opts.Backup, modules.ParseBackupOption, "sms"},
		{opts.Download, modules.ParseDownloadOption, "all"},
		{opts.RemoveTrusted, modules.ParseRemoveTrustedOption, "no"},
		{opts.IntrusionLogs, modules.ParseIntrusionLogsOption, "no"},
		{opts.HashFiles, modules.ParseHashFilesOption, "yes"},
		{opts.BrowserHistory, modules.ParseBrowserHistoryOption, "yes"},
	} {
		want, err := check.parse(check.token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if check.got != want {
			t.Fatalf("got %q, want %q", check.got, want)
		}
	}
}
