package modules

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestWaitForNewFilesConsumesInterruptAsSkip(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signals <- os.Interrupt

	err := NewIL().waitForNewFiles(context.Background(), signals, "/unused", nil, time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("waitForNewFiles() error = %v", err)
	}
}

func TestWaitForNewFilesPropagatesTermination(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM

	err := NewIL().waitForNewFiles(context.Background(), signals, "/unused", nil, time.Hour, time.Hour)
	if !errors.Is(err, ErrAcquisitionInterrupted) {
		t.Fatalf("waitForNewFiles() error = %v, want ErrAcquisitionInterrupted", err)
	}
}
