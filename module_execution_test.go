package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/mvt-project/androidqf/acquisition"
	"github.com/mvt-project/androidqf/adb"
	"github.com/mvt-project/androidqf/modules"
)

type contextBlockingModule struct {
	name    string
	started chan struct{}
}

func (m *contextBlockingModule) Name() string { return m.name }

func (m *contextBlockingModule) Run(_ *acquisition.Acquisition, opts *modules.Options) error {
	close(m.started)
	<-opts.ContextOrBackground().Done()
	return opts.ContextOrBackground().Err()
}

type interruptWaitingModule struct {
	started chan struct{}
}

func (m *interruptWaitingModule) Name() string { return modules.NewIL().Name() }

func (m *interruptWaitingModule) Run(_ *acquisition.Acquisition, opts *modules.Options) error {
	close(m.started)
	received := <-opts.Signals
	if received != os.Interrupt {
		return fmt.Errorf("received %v, want %v", received, os.Interrupt)
	}
	return nil
}

func TestRunModuleCancelsActiveModuleOnTermination(t *testing.T) {
	oldClient := adb.Client
	adb.Client = nil
	t.Cleanup(func() { adb.Client = oldClient })

	started := make(chan struct{})
	signals := make(chan os.Signal, 1)
	go func() {
		<-started
		signals <- syscall.SIGTERM
	}()

	opts := &modules.Options{}
	err, interrupted := runModule(
		&contextBlockingModule{name: "blocking", started: started},
		&acquisition.Acquisition{},
		opts,
		signals,
	)
	if !interrupted {
		t.Fatal("runModule() interrupted = false")
	}
	if !errors.Is(err, modules.ErrAcquisitionInterrupted) {
		t.Fatalf("runModule() error = %v, want ErrAcquisitionInterrupted", err)
	}
	if opts.Context != nil || opts.Signals != nil {
		t.Fatalf("module options retained runtime state: %+v", opts)
	}
}

func TestRunModuleForwardsFirstIntrusionLogInterrupt(t *testing.T) {
	oldClient := adb.Client
	adb.Client = nil
	t.Cleanup(func() { adb.Client = oldClient })

	started := make(chan struct{})
	signals := make(chan os.Signal, 1)
	go func() {
		<-started
		signals <- os.Interrupt
	}()

	err, interrupted := runModule(
		&interruptWaitingModule{started: started},
		&acquisition.Acquisition{},
		&modules.Options{},
		signals,
	)
	if err != nil {
		t.Fatalf("runModule() error = %v", err)
	}
	if interrupted {
		t.Fatal("runModule() interrupted = true")
	}
}

func TestModuleResultStatusDistinguishesPartialCollection(t *testing.T) {
	if got := moduleResultStatus(nil); got != "completed" {
		t.Fatalf("nil status = %q, want completed", got)
	}
	if got := moduleResultStatus(fmt.Errorf("%w: missing file", modules.ErrPartialCollection)); got != "partial" {
		t.Fatalf("partial status = %q, want partial", got)
	}
	interruptedPartial := errors.Join(
		fmt.Errorf("%w: missing file", modules.ErrPartialCollection),
		modules.ErrAcquisitionInterrupted,
	)
	if got := moduleResultStatus(interruptedPartial); got != "failed" {
		t.Fatalf("interrupted partial status = %q, want failed", got)
	}
}

func TestPrintAcquisitionSummary(t *testing.T) {
	acq := &acquisition.Acquisition{
		Completed: time.Date(2026, 11, 20, 14, 3, 20, 123456789, time.UTC),
		ModuleResults: []acquisition.ModuleResult{
			{Name: "backup", Status: "skipped"},
			{Name: "packages", Status: "completed"},
			{Name: "apex", Status: "partial"},
			{Name: "bugreport", Status: "failed"},
		},
	}
	const header = "Acquisition finalized at 2026-11-20T14:03:20.123456789Z\n"
	var out bytes.Buffer
	printAcquisitionSummary(&out, acq)
	if got, want := out.String(), header+"  backup: skipped\n  packages: completed\n  apex: partial\n  bugreport: failed\n"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}

	acq.ModuleResults = nil
	out.Reset()
	printAcquisitionSummary(&out, acq)
	if got, want := out.String(), header+"No modules ran.\n"; got != want {
		t.Fatalf("empty summary = %q, want %q", got, want)
	}
}

func TestDeclinedBackupStatus(t *testing.T) {
	backup, err := modules.ParseBackupOption("none")
	if err != nil {
		t.Fatal(err)
	}
	err = modules.NewBackup().Run(&acquisition.Acquisition{}, &modules.Options{Backup: backup, NonInteractive: true})
	if got := moduleResultStatus(err); got != "skipped" {
		t.Fatalf("declined backup status = %q, want skipped", got)
	}
	if got := moduleResultStatus(errors.Join(err, modules.ErrAcquisitionInterrupted)); got != "failed" {
		t.Fatalf("interrupted backup status = %q, want failed", got)
	}
}
