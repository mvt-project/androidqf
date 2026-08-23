// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/minio/sio"
	"github.com/mvt-project/androidqf/adb"
)

var ErrStreamingBufferMemoryLimit = errors.New("streaming buffer memory limit exceeded")

// StreamingBuffer manages in-memory buffering for direct streaming operations
type StreamingBuffer struct {
	buffer *bytes.Buffer
	size   int64
	maxMem int64
}

// NewStreamingBuffer creates a new streaming buffer with the specified max memory usage
func NewStreamingBuffer(maxMemoryMB int) *StreamingBuffer {
	return &StreamingBuffer{
		buffer: bytes.NewBuffer(nil),
		size:   0,
		maxMem: int64(maxMemoryMB) * 1024 * 1024,
	}
}

// Write implements io.Writer interface with memory limit enforcement
func (sb *StreamingBuffer) Write(p []byte) (int, error) {
	if sb.size+int64(len(p)) > sb.maxMem {
		return 0, fmt.Errorf("%w: write would exceed memory limit of %d bytes", ErrStreamingBufferMemoryLimit, sb.maxMem)
	}

	n, err := sb.buffer.Write(p)
	if err != nil {
		return n, err
	}
	sb.size += int64(n)
	return n, nil
}

// Reader returns a seekable reader over the buffered data without copying it.
func (sb *StreamingBuffer) Reader() *bytes.Reader {
	return bytes.NewReader(sb.buffer.Bytes())
}

// Bytes returns the buffered data as a byte slice
func (sb *StreamingBuffer) Bytes() []byte {
	return sb.buffer.Bytes()
}

// Size returns the current size of buffered data
func (sb *StreamingBuffer) Size() int64 {
	return sb.size
}

// Reset clears the buffer
func (sb *StreamingBuffer) Reset() {
	sb.buffer.Reset()
	sb.size = 0
}

// StreamingPuller provides utilities for streaming ADB operations
type StreamingPuller struct {
	adbPath string
	serial  string
	maxMem  int64
	ctxMu   sync.RWMutex
	ctx     context.Context
}

// EncryptedTempFile is a DARE-encrypted temporary file used to validate a
// device pull before its plaintext is written to the acquisition archive. SIO
// provides authenticated random access to support APK certificate verification.
type EncryptedTempFile struct {
	path          string
	key           [32]byte
	plaintextSize int64
}

type encryptedTempReader struct {
	*io.SectionReader
	file *os.File
}

func (r *encryptedTempReader) Close() error {
	return r.file.Close()
}

// Open returns an authenticated, seekable plaintext view of the encrypted
// staging file. The caller must close the returned reader.
func (f *EncryptedTempFile) Open() (io.ReadSeekCloser, error) {
	file, err := os.Open(f.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open encrypted temporary file: %w", err)
	}

	readerAt, err := sio.DecryptReaderAt(file, f.sioConfig())
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to decrypt temporary file: %w", err)
	}
	return &encryptedTempReader{
		SectionReader: io.NewSectionReader(readerAt, 0, f.plaintextSize),
		file:          file,
	}, nil
}

// Remove deletes the encrypted staging file.
func (f *EncryptedTempFile) Remove() error {
	err := os.Remove(f.path)
	clear(f.key[:])
	return err
}

func (f *EncryptedTempFile) sioConfig() sio.Config {
	return sio.Config{
		MinVersion:   sio.Version20,
		MaxVersion:   sio.Version20,
		CipherSuites: []byte{sio.CHACHA20_POLY1305},
		Key:          f.key[:],
	}
}

// NewStreamingPuller creates a new streaming puller
func NewStreamingPuller(adbPath, serial string, maxMemoryMB int) *StreamingPuller {
	return &StreamingPuller{
		adbPath: adbPath,
		serial:  serial,
		maxMem:  int64(maxMemoryMB) * 1024 * 1024,
		ctx:     context.Background(),
	}
}

// SetContext changes the context used by subsequent streaming ADB commands.
func (sp *StreamingPuller) SetContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	sp.ctxMu.Lock()
	sp.ctx = ctx
	sp.ctxMu.Unlock()
}

func (sp *StreamingPuller) command(args ...string) *exec.Cmd {
	sp.ctxMu.RLock()
	ctx := sp.ctx
	sp.ctxMu.RUnlock()
	if ctx == nil {
		ctx = context.Background()
	}
	return exec.CommandContext(ctx, sp.adbPath, args...)
}

// PullToBuffer pulls a file from device directly into memory buffer
func (sp *StreamingPuller) PullToBuffer(remotePath string) (*StreamingBuffer, error) {
	if remotePath == "" {
		return nil, fmt.Errorf("remote path cannot be empty")
	}

	buffer := NewStreamingBuffer(int(sp.maxMem / (1024 * 1024)))

	args := []string{"exec-out", "cat", adb.QuoteRemoteShellArg(remotePath)}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := sp.command(args...)
	cmd.Stdout = buffer

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to pull %q to buffer: %w", remotePath, err)
	}

	return buffer, nil
}

// PullToWriter pulls a file from device and streams it directly to a writer
func (sp *StreamingPuller) PullToWriter(remotePath string, writer io.Writer) error {
	return sp.pullToWriter(remotePath, writer, false)
}

// PullRootToWriter pulls a file that is only readable as root and streams it
// directly to a writer. It requires an already-functional su binary and never
// attempts to alter the device's root state.
func (sp *StreamingPuller) PullRootToWriter(remotePath string, writer io.Writer) error {
	return sp.pullToWriter(remotePath, writer, true)
}

func (sp *StreamingPuller) pullToWriter(remotePath string, writer io.Writer, root bool) error {
	if remotePath == "" {
		return fmt.Errorf("remote path cannot be empty")
	}
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	args := []string{"exec-out", "cat", adb.QuoteRemoteShellArg(remotePath)}
	if root {
		args = []string{"exec-out", "su", "-c", "cat -- " + adb.QuoteRemoteShellArg(remotePath)}
	}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := sp.command(args...)
	cmd.Stdout = writer

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to pull %q to writer: %v", remotePath, err)
	}

	return nil
}

// PullToTempFile pulls a file from the device into a temporary file and
// returns its path. The caller is responsible for removing the file.
func (sp *StreamingPuller) PullToTempFile(remotePath string) (string, error) {
	return sp.pullToTempFile(remotePath, false)
}

// PullRootToTempFile stages a root-readable device file in a host temporary
// file. The caller is responsible for removing the returned path.
func (sp *StreamingPuller) PullRootToTempFile(remotePath string) (string, error) {
	return sp.pullToTempFile(remotePath, true)
}

func (sp *StreamingPuller) pullToTempFile(remotePath string, root bool) (string, error) {
	tempFile, err := os.CreateTemp("", "androidqf-pull-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()

	pull := sp.PullToWriter
	if root {
		pull = sp.PullRootToWriter
	}
	if err := pull(remotePath, tempFile); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("failed to pull to temporary file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	return tempPath, nil
}

// PullToEncryptedTempFile pulls a device file into a ChaCha20-Poly1305 DARE
// temporary file. Plaintext is only streamed through memory and is never
// staged on disk.
func (sp *StreamingPuller) PullToEncryptedTempFile(remotePath string) (*EncryptedTempFile, error) {
	if remotePath == "" {
		return nil, fmt.Errorf("remote path cannot be empty")
	}
	return createEncryptedTempFile(func(writer io.Writer) error {
		return sp.PullToWriter(remotePath, writer)
	})
}

// PullRootToEncryptedTempFile stages a root-readable device file encrypted on
// the host. Plaintext is never written to host storage.
func (sp *StreamingPuller) PullRootToEncryptedTempFile(remotePath string) (*EncryptedTempFile, error) {
	if remotePath == "" {
		return nil, fmt.Errorf("remote path cannot be empty")
	}
	return createEncryptedTempFile(func(writer io.Writer) error {
		return sp.PullRootToWriter(remotePath, writer)
	})
}

func createEncryptedTempFile(writePlaintext func(io.Writer) error) (*EncryptedTempFile, error) {
	if writePlaintext == nil {
		return nil, fmt.Errorf("plaintext writer cannot be nil")
	}

	staged := &EncryptedTempFile{}
	if _, err := io.ReadFull(rand.Reader, staged.key[:]); err != nil {
		return nil, fmt.Errorf("failed to generate temporary encryption key: %w", err)
	}

	tempFile, err := os.CreateTemp("", "androidqf-pull-*.dare")
	if err != nil {
		clear(staged.key[:])
		return nil, fmt.Errorf("failed to create encrypted temporary file: %w", err)
	}
	staged.path = tempFile.Name()
	cleanup := func() {
		_ = tempFile.Close()
		_ = staged.Remove()
	}

	encryptedWriter, err := sio.EncryptWriter(tempFile, staged.sioConfig())
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create temporary encryption writer: %w", err)
	}

	if err := writePlaintext(encryptedWriter); err != nil {
		_ = encryptedWriter.Close()
		cleanup()
		return nil, fmt.Errorf("failed to pull to encrypted temporary file: %w", err)
	}
	if err := encryptedWriter.Close(); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to finalize encrypted temporary file: %w", err)
	}
	if err := tempFile.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		cleanup()
		return nil, fmt.Errorf("failed to close encrypted temporary file: %w", err)
	}

	stat, err := os.Stat(staged.path)
	if err != nil {
		_ = staged.Remove()
		return nil, fmt.Errorf("failed to stat encrypted temporary file: %w", err)
	}
	plaintextSize, err := sio.DecryptedSize(uint64(stat.Size()))
	if err != nil {
		_ = staged.Remove()
		return nil, fmt.Errorf("failed to determine encrypted temporary file size: %w", err)
	}
	staged.plaintextSize = int64(plaintextSize)

	return staged, nil
}

// BackupToBuffer creates a backup directly into memory buffer using exec-out
func (sp *StreamingPuller) BackupToBuffer(arg string) (*StreamingBuffer, error) {
	if arg == "" {
		return nil, fmt.Errorf("backup argument cannot be empty")
	}

	buffer := NewStreamingBuffer(int(sp.maxMem / (1024 * 1024)))

	args := []string{"exec-out", "bu", "backup", arg}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := sp.command(args...)
	cmd.Stdout = buffer

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to create backup %q to buffer: %v", arg, err)
	}

	return buffer, nil
}

// BackupToWriter creates a backup and streams it directly to a writer using exec-out
func (sp *StreamingPuller) BackupToWriter(arg string, writer io.Writer) error {
	if arg == "" {
		return fmt.Errorf("backup argument cannot be empty")
	}
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	args := []string{"exec-out", "bu", "backup", "-nocompress", arg}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := sp.command(args...)
	cmd.Stdout = writer

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create backup %q to writer: %v", arg, err)
	}

	return nil
}

// BugreportToBuffer creates a bugreport directly into memory buffer using bugreportz
func (sp *StreamingPuller) BugreportToBuffer() (*StreamingBuffer, error) {
	filename, err := sp.generateBugreport()
	if err != nil {
		return nil, err
	}

	// Ensure cleanup happens regardless of success/failure
	defer sp.cleanupDeviceFile(filename)

	// Stream the bugreport file to buffer
	buffer := NewStreamingBuffer(int(sp.maxMem / (1024 * 1024)))

	streamArgs := []string{"exec-out", "cat", adb.QuoteRemoteShellArg(filename)}
	if sp.serial != "" {
		streamArgs = append([]string{"-s", sp.serial}, streamArgs...)
	}

	streamCmd := sp.command(streamArgs...)
	streamCmd.Stdout = buffer

	err = streamCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to stream bugreport file: %v", err)
	}

	return buffer, nil
}

// BugreportToWriter creates a bugreport and streams it directly to a writer using bugreportz
func (sp *StreamingPuller) BugreportToWriter(writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	filename, err := sp.generateBugreport()
	if err != nil {
		return err
	}

	// Ensure cleanup happens regardless of success/failure
	defer sp.cleanupDeviceFile(filename)

	// Stream the bugreport file to writer
	streamArgs := []string{"exec-out", "cat", adb.QuoteRemoteShellArg(filename)}
	if sp.serial != "" {
		streamArgs = append([]string{"-s", sp.serial}, streamArgs...)
	}

	streamCmd := sp.command(streamArgs...)
	streamCmd.Stdout = writer

	err = streamCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to stream bugreport file: %v", err)
	}

	return nil
}

// generateBugreport generates a bugreport on device and returns the filename
func (sp *StreamingPuller) generateBugreport() (string, error) {
	args := []string{"shell", "bugreportz"}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := sp.command(args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate bugreport with bugreportz: %v", err)
	}

	// Parse output to get filename (bugreportz outputs: OK:/data/user_de/0/com.android.shell/files/bugreports/bugreport-xxx.zip)
	filename := strings.TrimSpace(string(output))
	if strings.HasPrefix(filename, "OK:") {
		filename = strings.TrimPrefix(filename, "OK:")
	} else {
		return "", fmt.Errorf("bugreportz failed: %s", filename)
	}

	return filename, nil
}

// cleanupDeviceFile removes a file from the device
func (sp *StreamingPuller) cleanupDeviceFile(filename string) {
	if filename == "" {
		return
	}

	cleanupArgs := []string{"shell", "rm", adb.QuoteRemoteShellArg(filename)}
	if sp.serial != "" {
		cleanupArgs = append([]string{"-s", sp.serial}, cleanupArgs...)
	}
	cleanupCmd := exec.Command(sp.adbPath, cleanupArgs...)
	cleanupCmd.Run() // Ignore errors for cleanup
}
