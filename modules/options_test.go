package modules

import (
	"strings"
	"testing"
)

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
		{"interactive ignores unknown module", &Options{}, "typo", nil, nil},
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
			[]string{"-backup", "-download", "-intrusion-logs"},
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
			&Options{NonInteractive: true, Backup: backupNothing, Download: apkNone, IntrusionLogs: skipIL},
			"",
			nil,
			nil,
		},
		{
			"module filter narrows requirements",
			&Options{NonInteractive: true},
			"backup",
			[]string{"-backup"},
			[]string{"-download", "-intrusion-logs"},
		},
		{
			"all set",
			&Options{NonInteractive: true, Backup: backupOnlySMS, Download: apkAll, RemoveTrusted: apkKeepAll, IntrusionLogs: skipIL},
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
