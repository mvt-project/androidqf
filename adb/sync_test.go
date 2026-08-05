package adb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestSyncPullToWriter(t *testing.T) {
	address := startFakeADBServer(t, func(conn net.Conn) error {
		if err := expectHostRequest(conn, "host:transport:device-1"); err != nil {
			return err
		}
		if err := expectHostRequest(conn, "sync:"); err != nil {
			return err
		}
		if err := expectSyncRequest(conn, "RECV", "/sys/fs/selinux/policy"); err != nil {
			return err
		}
		if err := writeSyncFrame(conn, "DATA", []byte("active ")); err != nil {
			return err
		}
		if err := writeSyncFrame(conn, "DATA", []byte("policy")); err != nil {
			return err
		}
		return writeSyncFrame(conn, "DONE", nil)
	})

	client := &ADB{Serial: "device-1", serverAddress: address}
	var output bytes.Buffer
	if err := client.SyncPullToWriter("/sys/fs/selinux/policy", &output); err != nil {
		t.Fatalf("SyncPullToWriter() error = %v", err)
	}
	if got := output.String(); got != "active policy" {
		t.Fatalf("SyncPullToWriter() output = %q, want %q", got, "active policy")
	}
}

func TestSyncPullToWriterUsesAnyTransportWithoutSerial(t *testing.T) {
	address := startFakeADBServer(t, func(conn net.Conn) error {
		if err := expectHostRequest(conn, "host:transport-any"); err != nil {
			return err
		}
		if err := expectHostRequest(conn, "sync:"); err != nil {
			return err
		}
		if err := expectSyncRequest(conn, "RECV", "/vendor/policy"); err != nil {
			return err
		}
		return writeSyncFrame(conn, "DONE", nil)
	})

	client := &ADB{serverAddress: address}
	if err := client.SyncPullToWriter("/vendor/policy", io.Discard); err != nil {
		t.Fatalf("SyncPullToWriter() error = %v", err)
	}
}

func TestSyncPullToWriterReportsHostFailure(t *testing.T) {
	address := startFakeADBServer(t, func(conn net.Conn) error {
		request, err := readHostRequest(conn)
		if err != nil {
			return err
		}
		if request != "host:transport:missing" {
			return fmt.Errorf("host request = %q", request)
		}
		if _, err := io.WriteString(conn, "FAIL0010device not found"); err != nil {
			return err
		}
		return nil
	})

	client := &ADB{Serial: "missing", serverAddress: address}
	err := client.SyncPullToWriter("/policy", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "device not found") {
		t.Fatalf("SyncPullToWriter() error = %v, want device-not-found failure", err)
	}
}

func TestSyncPullToWriterReportsSyncFailure(t *testing.T) {
	address := startFakeADBServer(t, func(conn net.Conn) error {
		if err := expectHostRequest(conn, "host:transport-any"); err != nil {
			return err
		}
		if err := expectHostRequest(conn, "sync:"); err != nil {
			return err
		}
		if err := expectSyncRequest(conn, "RECV", "/missing"); err != nil {
			return err
		}
		return writeSyncFrame(conn, "FAIL", []byte("No such file"))
	})

	client := &ADB{serverAddress: address}
	err := client.SyncPullToWriter("/missing", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "No such file") {
		t.Fatalf("SyncPullToWriter() error = %v, want missing-file failure", err)
	}
}

func TestReceiveSyncFileRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "unknown response",
			data: syncFrame("NOPE", nil),
		},
		{
			name: "oversized data",
			data: syncHeader("DATA", maxSyncFrameSize+1),
		},
		{
			name: "oversized failure",
			data: syncHeader("FAIL", maxSyncFrameSize+1),
		},
		{
			name: "truncated data",
			data: append(syncHeader("DATA", 8), []byte("short")...),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := receiveSyncFile(bytes.NewReader(tt.data), io.Discard); err == nil {
				t.Fatal("receiveSyncFile() error = nil, want error")
			}
		})
	}
}

func TestReceiveSyncFilePropagatesWriterFailure(t *testing.T) {
	input := append(syncFrame("DATA", []byte("policy")), syncFrame("DONE", nil)...)
	wantErr := errors.New("write failed")
	writer := errorWriter{err: wantErr}

	err := receiveSyncFile(bytes.NewReader(input), writer)
	if !errors.Is(err, wantErr) {
		t.Fatalf("receiveSyncFile() error = %v, want %v", err, wantErr)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func startFakeADBServer(t *testing.T, serve func(net.Conn) error) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		done <- serve(conn)
	}()
	t.Cleanup(func() {
		if err := <-done; err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("fake ADB server error: %v", err)
		}
	})

	return listener.Addr().String()
}

func expectHostRequest(conn net.Conn, want string) error {
	got, err := readHostRequest(conn)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("host request = %q, want %q", got, want)
	}
	_, err = io.WriteString(conn, "OKAY")
	return err
}

func readHostRequest(reader io.Reader) (string, error) {
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBytes); err != nil {
		return "", err
	}
	length, err := strconv.ParseUint(string(lengthBytes), 16, 16)
	if err != nil {
		return "", err
	}
	request := make([]byte, int(length))
	if _, err := io.ReadFull(reader, request); err != nil {
		return "", err
	}
	return string(request), nil
}

func expectSyncRequest(reader io.Reader, wantID, wantPath string) error {
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	if got := string(header[:4]); got != wantID {
		return fmt.Errorf("sync request ID = %q, want %q", got, wantID)
	}
	length := binary.LittleEndian.Uint32(header[4:])
	path := make([]byte, int(length))
	if _, err := io.ReadFull(reader, path); err != nil {
		return err
	}
	if got := string(path); got != wantPath {
		return fmt.Errorf("sync request path = %q, want %q", got, wantPath)
	}
	return nil
}

func writeSyncFrame(writer io.Writer, id string, data []byte) error {
	if err := writeBytes(writer, syncHeader(id, len(data))); err != nil {
		return err
	}
	return writeBytes(writer, data)
}

func syncFrame(id string, data []byte) []byte {
	return append(syncHeader(id, len(data)), data...)
}

func syncHeader(id string, length int) []byte {
	header := make([]byte, 8)
	copy(header[:4], id)
	binary.LittleEndian.PutUint32(header[4:], uint32(length))
	return header
}
