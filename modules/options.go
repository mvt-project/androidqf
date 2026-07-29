// androidqf - Android Quick Forensics
// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"fmt"
	"strings"
)

// Options carries per-run module configuration. An empty string field means
// no answer was provided on the command line, so the module prompts
// interactively unless NonInteractive is set.
type Options struct {
	Fast           bool
	NonInteractive bool
	Backup         string
	Download       string
	RemoveTrusted  string
	IntrusionLogs  string
	HashFiles      string
}

func ModuleEnabled(name, filter string) bool {
	return filter == "" || filter == name
}

func moduleExists(name string) bool {
	for _, mod := range List() {
		if mod.Name() == name {
			return true
		}
	}
	return false
}

func ValidateNonInteractive(opts *Options, moduleFilter string) error {
	if moduleFilter != "" && !moduleExists(moduleFilter) {
		return fmt.Errorf("unknown -module value %q, use -list to see available modules", moduleFilter)
	}
	if opts == nil || !opts.NonInteractive {
		return nil
	}

	var missing []string
	if ModuleEnabled(NewBackup().Name(), moduleFilter) && opts.Backup == "" {
		missing = append(missing, "-backup (sms, all, none)")
	}
	if ModuleEnabled(NewPackages().Name(), moduleFilter) {
		if opts.Download == "" {
			missing = append(missing, "-download (all, non-system, none)")
		} else if opts.Download != apkNone && opts.RemoveTrusted == "" {
			missing = append(missing, "-remove-trusted (yes, no)")
		}
	}
	if ModuleEnabled(NewIL().Name(), moduleFilter) && opts.IntrusionLogs == "" {
		missing = append(missing, "-intrusion-logs (yes, no)")
	}
	if ModuleEnabled(NewFiles().Name(), moduleFilter) && opts.HashFiles == "" {
		missing = append(missing, "-hash-files (yes, no)")
	}

	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("-non-interactive requires: %s", strings.Join(missing, ", "))
}

func resolveOption(opts *Options, value, flagUsage string, prompt func() (string, error)) (string, error) {
	if value != "" {
		return value, nil
	}
	if opts.NonInteractive {
		return "", fmt.Errorf("-non-interactive is set but %s was not provided", flagUsage)
	}
	return prompt()
}
