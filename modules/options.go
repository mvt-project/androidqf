// androidqf - Android Quick Forensics
// Copyright (c) 2021-2026 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrAcquisitionInterrupted = errors.New("acquisition interrupted")
var ErrPartialCollection = errors.New("partial collection")

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
	BrowserHistory string
	MagiskModules  string
	Signals        <-chan os.Signal
	Context        context.Context
}

func (o *Options) ContextOrBackground() context.Context {
	if o == nil || o.Context == nil {
		return context.Background()
	}
	return o.Context
}

func partialCollectionError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrPartialCollection, err)
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
	if ModuleEnabled(NewBrowserHistory().Name(), moduleFilter) && opts.BrowserHistory == "" {
		missing = append(missing, "-browser-history (yes, no)")
	}
	if ModuleEnabled(NewMagiskModules().Name(), moduleFilter) && opts.MagiskModules == "" {
		missing = append(missing, "-magisk-modules (yes, no)")
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

	type promptResult struct {
		value string
		err   error
	}
	result := make(chan promptResult, 1)
	go func() {
		value, err := prompt()
		result <- promptResult{value: value, err: err}
	}()

	ctx := opts.ContextOrBackground()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("%w: %v", ErrAcquisitionInterrupted, ctx.Err())
	case resolved := <-result:
		return resolved.value, resolved.err
	}
}
