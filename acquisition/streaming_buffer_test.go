package acquisition

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as a fake adb executable")
	}

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

func TestPullToWriterCancelsActiveADBCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as a fake adb executable")
	}

	fakeADB := filepath.Join(t.TempDir(), "adb")
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("ANDROIDQF_FAKE_ADB_MARKER", marker)
	if err := os.WriteFile(fakeADB, []byte("#!/bin/sh\n: > \"$ANDROIDQF_FAKE_ADB_MARKER\"\nexec sleep 3600\n"), 0o700); err != nil {
		t.Fatalf("WriteFile(fake adb) error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	puller := NewStreamingPuller(fakeADB, "", 1)
	puller.SetContext(ctx)
	writer := new(bytes.Buffer)
	done := make(chan error, 1)
	go func() {
		done <- puller.PullToWriter("/data/local/tmp/file", writer)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("fake adb stopped before cancellation: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("fake adb did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("PullToWriter() error = nil after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PullToWriter() did not stop after cancellation")
	}
}

func TestEncryptedTempFileDoesNotStagePlaintextAndSupportsSeeking(t *testing.T) {
	content := bytes.Repeat([]byte("sensitive APK content 0123456789\n"), 4096)
	staged, err := createEncryptedTempFile(func(writer io.Writer) error {
		_, err := writer.Write(content)
		return err
	})
	if err != nil {
		t.Fatalf("createEncryptedTempFile() error = %v", err)
	}
	defer staged.Remove()

	raw, err := os.ReadFile(staged.path)
	if err != nil {
		t.Fatalf("ReadFile(encrypted stage) error = %v", err)
	}
	if bytes.Contains(raw, content[:64]) {
		t.Fatal("encrypted temporary file contains plaintext content")
	}

	reader, err := staged.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := reader.Seek(70_000, io.SeekStart); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	decrypted := make([]byte, 512)
	_, err = io.ReadFull(reader, decrypted)
	reader.Close()
	if err != nil {
		t.Fatalf("ReadFull(decrypted stage) error = %v", err)
	}
	if !bytes.Equal(decrypted, content[70_000:70_512]) {
		t.Fatal("random-access decrypted content does not match plaintext")
	}
}

func TestEncryptedTempFileClosesStagingDescriptor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("checks Linux process file descriptors")
	}

	staged, err := createEncryptedTempFile(func(writer io.Writer) error {
		_, err := io.WriteString(writer, "encrypted staging content")
		return err
	})
	if err != nil {
		t.Fatalf("createEncryptedTempFile() error = %v", err)
	}
	defer staged.Remove()

	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("ReadDir(/proc/self/fd) error = %v", err)
	}
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name()))
		if err == nil && target == staged.path {
			t.Fatalf("encrypted staging file descriptor %s is still open", fd.Name())
		}
	}
}

func TestEncryptedTempFileRejectsTampering(t *testing.T) {
	staged, err := createEncryptedTempFile(func(writer io.Writer) error {
		_, err := io.WriteString(writer, strings.Repeat("authenticated content", 10_000))
		return err
	})
	if err != nil {
		t.Fatalf("createEncryptedTempFile() error = %v", err)
	}
	defer staged.Remove()

	file, err := os.OpenFile(staged.path, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	stat, err := file.Stat()
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	last := make([]byte, 1)
	if _, err := file.ReadAt(last, stat.Size()-1); err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	last[0] ^= 0xff
	if _, err := file.WriteAt(last, stat.Size()-1); err != nil {
		t.Fatalf("WriteAt() error = %v", err)
	}
	file.Close()

	reader, err := staged.Open()
	if err == nil {
		_, err = io.ReadAll(reader)
		reader.Close()
	}
	if err == nil {
		t.Fatal("tampered encrypted temporary file was accepted")
	}
}

func TestPullToZipStagedDoesNotCreateEntryForFailedPull(t *testing.T) {
	fakeADB := filepath.Join(t.TempDir(), "missing-adb.exe")
	if runtime.GOOS != "windows" {
		fakeADB = filepath.Join(t.TempDir(), "adb")
		if err := os.WriteFile(fakeADB, []byte("#!/bin/sh\nprintf 'partial evidence'\nexit 1\n"), 0o700); err != nil {
			t.Fatalf("WriteFile(fake adb) error = %v", err)
		}
	}

	outputDir := t.TempDir()
	writer, err := NewStreamingZipWriter("failed-pull", outputDir)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	defer writer.Close()
	acq := &Acquisition{
		ZipWriter:       writer,
		StreamingMode:   true,
		StreamingPuller: NewStreamingPuller(fakeADB, "", 1),
	}

	if err := acq.PullToZipStaged("/proc/kmsg", "logs/proc/kmsg"); err == nil {
		t.Fatal("PullToZipStaged() error = nil, want failed pull error")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reader, err := zip.OpenReader(writer.GetOutputPath())
	if err != nil {
		t.Fatalf("zip.OpenReader() error = %v", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if strings.EqualFold(file.Name, "logs/proc/kmsg") {
			t.Fatalf("archive contains entry for failed pull: %q", file.Name)
		}
	}
}

func TestPullRootToWriterUsesSuAndQuotesPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-specific")
	}

	fakeADB := filepath.Join(t.TempDir(), "adb")
	script := `#!/bin/sh
[ "$1" = "-s" ] || exit 2
[ "$2" = "serial-1" ] || exit 2
[ "$3" = "exec-out" ] || exit 2
[ "$4" = "su" ] || exit 2
[ "$5" = "-c" ] || exit 2
[ "$6" = "cat -- '/data/data/example'\"'\"'s/History'" ] || exit 2
printf 'history data'
`
	if err := os.WriteFile(fakeADB, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile(fake adb) error = %v", err)
	}

	var output bytes.Buffer
	puller := NewStreamingPuller(fakeADB, "serial-1", 1)
	if err := puller.PullRootToWriter("/data/data/example's/History", &output); err != nil {
		t.Fatalf("PullRootToWriter() error = %v", err)
	}
	if got := output.String(); got != "history data" {
		t.Fatalf("output = %q, want history data", got)
	}
}
