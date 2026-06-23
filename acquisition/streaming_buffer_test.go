package acquisition

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamingBufferMemoryLimitError(t *testing.T) {
	buffer := NewStreamingBuffer(1)

	if _, err := buffer.Write(make([]byte, 1024*1024)); err != nil {
		t.Fatalf("Write() at memory limit returned error: %v", err)
	}

	_, err := buffer.Write([]byte("x"))
	if !errors.Is(err, ErrStreamingBufferMemoryLimit) {
		t.Fatalf("Write() error = %v, want ErrStreamingBufferMemoryLimit", err)
	}
}

func TestDefaultStreamingPullerMemoryLimit(t *testing.T) {
	if streamingPullerMemoryLimitMB != 500 {
		t.Fatalf("streamingPullerMemoryLimitMB = %d, want 500", streamingPullerMemoryLimitMB)
	}
}

func TestPullToBufferPreservesMemoryLimitError(t *testing.T) {
	fakeADB := filepath.Join(t.TempDir(), "adb")
	if err := os.WriteFile(fakeADB, []byte("#!/bin/sh\nhead -c 1048577 /dev/zero\n"), 0o700); err != nil {
		t.Fatalf("WriteFile(fake adb) error = %v", err)
	}

	puller := NewStreamingPuller(fakeADB, "", 1)
	_, err := puller.PullToBuffer("/data/app/large.apk")
	if !errors.Is(err, ErrStreamingBufferMemoryLimit) {
		t.Fatalf("PullToBuffer() error = %v, want ErrStreamingBufferMemoryLimit", err)
	}
}
