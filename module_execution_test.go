package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

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
