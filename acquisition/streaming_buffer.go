// androidqf - Android Quick Forensics
// Copyright (c) 2021-2022 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package acquisition

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

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

// Write implements io.Writer interface
func (sb *StreamingBuffer) Write(p []byte) (int, error) {
	n, err := sb.buffer.Write(p)
	sb.size += int64(n)
	return n, err
}

// Reader returns an io.Reader for the buffered data
func (sb *StreamingBuffer) Reader() io.Reader {
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
}

// NewStreamingPuller creates a new streaming puller
func NewStreamingPuller(adbPath, serial string, maxMemoryMB int) *StreamingPuller {
	return &StreamingPuller{
		adbPath: adbPath,
		serial:  serial,
		maxMem:  int64(maxMemoryMB) * 1024 * 1024,
	}
}

// PullToBuffer pulls a file from device directly into memory buffer
func (sp *StreamingPuller) PullToBuffer(remotePath string) (*StreamingBuffer, error) {
	buffer := NewStreamingBuffer(int(sp.maxMem / (1024 * 1024)))

	args := []string{"exec-out", "cat", remotePath}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := exec.Command(sp.adbPath, args...)
	cmd.Stdout = buffer

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to pull %s to buffer: %v", remotePath, err)
	}

	return buffer, nil
}

// PullToWriter pulls a file from device and streams it directly to a writer
func (sp *StreamingPuller) PullToWriter(remotePath string, writer io.Writer) error {
	args := []string{"exec-out", "cat", remotePath}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := exec.Command(sp.adbPath, args...)
	cmd.Stdout = writer

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to pull %s to writer: %v", remotePath, err)
	}

	return nil
}

// BackupToBuffer creates a backup directly into memory buffer using exec-out
func (sp *StreamingPuller) BackupToBuffer(arg string) (*StreamingBuffer, error) {
	buffer := NewStreamingBuffer(int(sp.maxMem / (1024 * 1024)))

	args := []string{"exec-out", "bu", "backup", arg}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := exec.Command(sp.adbPath, args...)
	cmd.Stdout = buffer

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to create backup to buffer: %v", err)
	}

	return buffer, nil
}

// BackupToWriter creates a backup and streams it directly to a writer using exec-out
func (sp *StreamingPuller) BackupToWriter(arg string, writer io.Writer) error {
	args := []string{"exec-out", "bu", "backup", arg}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := exec.Command(sp.adbPath, args...)
	cmd.Stdout = writer

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create backup to writer: %v", err)
	}

	return nil
}

// BugreportToBuffer creates a bugreport directly into memory buffer using bugreportz
func (sp *StreamingPuller) BugreportToBuffer() (*StreamingBuffer, error) {
	// First, generate bugreport zip on device using bugreportz
	args := []string{"shell", "bugreportz"}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := exec.Command(sp.adbPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to generate bugreport with bugreportz: %v", err)
	}

	// Parse output to get filename (bugreportz outputs: OK:/data/user_de/0/com.android.shell/files/bugreports/bugreport-xxx.zip)
	filename := strings.TrimSpace(string(output))
	if strings.HasPrefix(filename, "OK:") {
		filename = strings.TrimPrefix(filename, "OK:")
	} else {
		return nil, fmt.Errorf("bugreportz failed: %s", filename)
	}

	// Stream the bugreport file to buffer
	buffer := NewStreamingBuffer(int(sp.maxMem / (1024 * 1024)))

	streamArgs := []string{"exec-out", "cat", filename}
	if sp.serial != "" {
		streamArgs = append([]string{"-s", sp.serial}, streamArgs...)
	}

	streamCmd := exec.Command(sp.adbPath, streamArgs...)
	streamCmd.Stdout = buffer

	err = streamCmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to stream bugreport file: %v", err)
	}

	// Clean up the bugreport file from device
	cleanupArgs := []string{"shell", "rm", filename}
	if sp.serial != "" {
		cleanupArgs = append([]string{"-s", sp.serial}, cleanupArgs...)
	}
	cleanupCmd := exec.Command(sp.adbPath, cleanupArgs...)
	cleanupCmd.Run() // Ignore errors for cleanup

	return buffer, nil
}

// BugreportToWriter creates a bugreport and streams it directly to a writer using bugreportz
func (sp *StreamingPuller) BugreportToWriter(writer io.Writer) error {
	// First, generate bugreport zip on device using bugreportz
	args := []string{"shell", "bugreportz"}
	if sp.serial != "" {
		args = append([]string{"-s", sp.serial}, args...)
	}

	cmd := exec.Command(sp.adbPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate bugreport with bugreportz: %v", err)
	}

	// Parse output to get filename (bugreportz outputs: OK:/data/user_de/0/com.android.shell/files/bugreports/bugreport-xxx.zip)
	filename := strings.TrimSpace(string(output))
	if strings.HasPrefix(filename, "OK:") {
		filename = strings.TrimPrefix(filename, "OK:")
	} else {
		return fmt.Errorf("bugreportz failed: %s", filename)
	}

	// Stream the bugreport file to writer
	streamArgs := []string{"exec-out", "cat", filename}
	if sp.serial != "" {
		streamArgs = append([]string{"-s", sp.serial}, streamArgs...)
	}

	streamCmd := exec.Command(sp.adbPath, streamArgs...)
	streamCmd.Stdout = writer

	err = streamCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to stream bugreport file: %v", err)
	}

	// Clean up the bugreport file from device
	cleanupArgs := []string{"shell", "rm", filename}
	if sp.serial != "" {
		cleanupArgs = append([]string{"-s", sp.serial}, cleanupArgs...)
	}
	cleanupCmd := exec.Command(sp.adbPath, cleanupArgs...)
	cleanupCmd.Run() // Ignore errors for cleanup

	return nil
}
