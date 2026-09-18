package modules

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPartialCollectionError(t *testing.T) {
	want := errors.New("missing evidence")
	err := partialCollectionError(want)
	if !errors.Is(err, ErrPartialCollection) {
		t.Fatalf("partialCollectionError() = %v, want ErrPartialCollection", err)
	}
	if err == nil || !strings.Contains(err.Error(), want.Error()) {
		t.Fatalf("partialCollectionError() = %v, want original detail", err)
	}
}

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name    string
		parse   func(string) (string, error)
		value   string
		want    string
		wantErr string
	}{
		{"backup sms", ParseBackupOption, "sms", backupOnlySMS, ""},
		{"backup all", ParseBackupOption, "all", backupEverything, ""},
		{"backup none", ParseBackupOption, "none", backupNothing, ""},
		{"backup mixed case", ParseBackupOption, " SMS ", backupOnlySMS, ""},
		{"backup invalid", ParseBackupOption, "maybe", "", "invalid -backup value"},
		{"download all", ParseDownloadOption, "all", apkAll, ""},
		{"download non-system", ParseDownloadOption, "non-system", apkNotSystem, ""},
		{"download none", ParseDownloadOption, "none", apkNone, ""},
		{"download invalid", ParseDownloadOption, "some", "", "invalid -download value"},
		{"remove-trusted yes", ParseRemoveTrustedOption, "yes", apkRemoveTrusted, ""},
		{"remove-trusted no", ParseRemoveTrustedOption, "no", apkKeepAll, ""},
		{"remove-trusted invalid", ParseRemoveTrustedOption, "nope", "", "invalid -remove-trusted value"},
		{"intrusion-logs yes", ParseIntrusionLogsOption, "yes", acquireIL, ""},
		{"intrusion-logs no", ParseIntrusionLogsOption, "no", skipIL, ""},
		{"intrusion-logs invalid", ParseIntrusionLogsOption, "never", "", "invalid -intrusion-logs value"},
		{"hash-files yes", ParseHashFilesOption, "yes", hashFiles, ""},
		{"hash-files no", ParseHashFilesOption, "no", skipHashes, ""},
		{"hash-files invalid", ParseHashFilesOption, "maybe", "", "invalid -hash-files value"},
		{"browser-history yes", ParseBrowserHistoryOption, "yes", acquireBrowserHistory, ""},
		{"browser-history no", ParseBrowserHistoryOption, "no", skipBrowserHistory, ""},
		{"browser-history invalid", ParseBrowserHistoryOption, "maybe", "", "invalid -browser-history value"},
		{"magisk-modules yes", ParseMagiskModulesOption, "yes", acquireMagiskModules, ""},
		{"magisk-modules no", ParseMagiskModulesOption, "no", skipMagiskModules, ""},
		{"magisk-modules invalid", ParseMagiskModulesOption, "maybe", "", "invalid -magisk-modules value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.parse(tt.value)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveOptionPresetSkipsPrompt(t *testing.T) {
	called := false
	got, err := resolveOption(&Options{NonInteractive: true}, backupNothing, "-backup", func() (string, error) {
		called = true
		return "", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != backupNothing {
		t.Fatalf("got %q, want %q", got, backupNothing)
	}
	if called {
		t.Fatal("prompt was called for preset value")
	}
}

func TestResolveOptionNonInteractiveErrors(t *testing.T) {
	called := false
	_, err := resolveOption(&Options{NonInteractive: true}, "", "-backup (sms, all, none)", func() (string, error) {
		called = true
		return "", nil
	})
	if err == nil || !strings.Contains(err.Error(), "-backup") {
		t.Fatalf("err = %v, want error mentioning -backup", err)
	}
	if called {
		t.Fatal("prompt was called in non-interactive mode")
	}
}

func TestResolveOptionInteractivePrompts(t *testing.T) {
	got, err := resolveOption(&Options{}, "", "-backup", func() (string, error) {
		return backupOnlySMS, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != backupOnlySMS {
		t.Fatalf("got %q, want %q", got, backupOnlySMS)
	}
}

func TestResolveOptionStopsWaitingWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		<-started
		cancel()
	}()

	_, err := resolveOption(&Options{Context: ctx}, "", "-backup", func() (string, error) {
		close(started)
		<-release
		return backupOnlySMS, nil
	})
	close(release)
	if !errors.Is(err, ErrAcquisitionInterrupted) {
		t.Fatalf("resolveOption() error = %v, want ErrAcquisitionInterrupted", err)
	}
}

func TestModuleEnabled(t *testing.T) {
	if !ModuleEnabled("backup", "") {
		t.Fatal("empty filter should enable every module")
	}
	if !ModuleEnabled("backup", "backup") {
		t.Fatal("matching filter should enable the module")
	}
	if ModuleEnabled("backup", "packages") {
		t.Fatal("non-matching filter should disable the module")
	}
}

func TestValidateNonInteractive(t *testing.T) {
	tests := []struct {
		name        string
		opts        *Options
		filter      string
		wantErr     []string
		wantMissing []string
	}{
		{"interactive", &Options{}, "", nil, nil},
		{
			"interactive rejects unknown module",
			&Options{},
			"typo",
			[]string{"unknown -module value"},
			nil,
		},
		{
			"unknown module filter",
			&Options{NonInteractive: true},
			"typo",
			[]string{"unknown -module value"},
			nil,
		},
		{
			"nothing set",
			&Options{NonInteractive: true},
			"",
			[]string{"-backup", "-download", "-intrusion-logs", "-hash-files", "-browser-history", "-magisk-modules"},
			[]string{"-remove-trusted"},
		},
		{
			"download set requires remove-trusted",
			&Options{NonInteractive: true, Download: apkAll},
			"",
			[]string{"-remove-trusted"},
			[]string{"-download"},
		},
		{
			"download none skips remove-trusted",
			&Options{NonInteractive: true, Backup: backupNothing, Download: apkNone, IntrusionLogs: skipIL, HashFiles: skipHashes, BrowserHistory: skipBrowserHistory, MagiskModules: skipMagiskModules},
			"",
			nil,
			nil,
		},
		{
			"module filter narrows requirements",
			&Options{NonInteractive: true},
			"backup",
			[]string{"-backup"},
			[]string{"-download", "-intrusion-logs", "-hash-files", "-browser-history", "-magisk-modules"},
		},
		{
			"files module requires hash choice",
			&Options{NonInteractive: true},
			"files",
			[]string{"-hash-files"},
			[]string{"-backup", "-download", "-intrusion-logs", "-browser-history", "-magisk-modules"},
		},
		{
			"files module accepts hash choice",
			&Options{NonInteractive: true, HashFiles: skipHashes},
			"files",
			nil,
			nil,
		},
		{
			"browser history module requires choice",
			&Options{NonInteractive: true},
			"browser_history",
			[]string{"-browser-history"},
			[]string{"-backup", "-download", "-intrusion-logs", "-hash-files", "-magisk-modules"},
		},
		{
			"magisk modules module requires choice",
			&Options{NonInteractive: true},
			"magisk_modules",
			[]string{"-magisk-modules"},
			[]string{"-backup", "-download", "-intrusion-logs", "-hash-files", "-browser-history"},
		},
		{
			"all set",
			&Options{NonInteractive: true, Backup: backupOnlySMS, Download: apkAll, RemoveTrusted: apkKeepAll, IntrusionLogs: skipIL, HashFiles: hashFiles, BrowserHistory: acquireBrowserHistory, MagiskModules: acquireMagiskModules},
			"",
			nil,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonInteractive(tt.opts, tt.filter)
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error mentioning %v, got nil", tt.wantErr)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %v, want containing %q", err, want)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(err.Error(), missing) {
					t.Fatalf("err = %v, must not contain %q", err, missing)
				}
			}
		})
	}
}
