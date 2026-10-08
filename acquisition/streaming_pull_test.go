package acquisition

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvt-project/androidqf/adb"
)

// Use the test executable as a portable fake adb. exec-out preserves each argv
// element as one remote argument, so reading the supplied path catches extra
// quoting without requiring a device or a POSIX shell on the host.
func TestMain(m *testing.M) {
	if os.Getenv("ANDROIDQF_STREAMING_ADB_HELPER") == "1" {
		if err := runStreamingADBHelper(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runStreamingADBHelper() error {
	args := os.Args[1:]
	if serial := os.Getenv("ANDROIDQF_STREAMING_ADB_SERIAL"); serial != "" {
		if len(args) < 2 || args[0] != "-s" || args[1] != serial {
			return fmt.Errorf("missing device serial in %q", args)
		}
		args = args[2:]
	}
	remotePath := os.Getenv("ANDROIDQF_STREAMING_ADB_PATH")
	switch {
	case len(args) == 2 && args[0] == "shell" && args[1] == "bugreportz":
		_, err := fmt.Fprintf(os.Stdout, "OK:%s\n", remotePath)
		return err
	case len(args) == 3 && args[0] == "shell" && args[1] == "rm":
		if args[2] != adb.QuoteRemoteShellArg(remotePath) {
			return fmt.Errorf("cleanup path is not shell-quoted: %q", args[2])
		}
		return os.WriteFile(os.Getenv("ANDROIDQF_STREAMING_ADB_CLEANUP"), []byte("cleaned"), 0o600)
	case len(args) == 4 && args[0] == "exec-out" && args[1] == "cat" && args[2] == "--":
		content, err := os.ReadFile(args[3])
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(content)
		return err
	default:
		return fmt.Errorf("unexpected adb arguments: %q", args)
	}
}

func TestStreamingPullsPreserveFileContents(t *testing.T) {
	methods := []struct {
		name      string
		bugreport bool
		pull      func(*StreamingPuller, string) ([]byte, error)
	}{
		{"buffer", false, func(sp *StreamingPuller, path string) ([]byte, error) {
			buffer, err := sp.PullToBuffer(path)
			if err != nil {
				return nil, err
			}
			return buffer.Bytes(), nil
		}},
		{"writer", false, func(sp *StreamingPuller, path string) ([]byte, error) {
			var buffer bytes.Buffer
			err := sp.PullToWriter(path, &buffer)
			return buffer.Bytes(), err
		}},
		{"bugreport-buffer", true, func(sp *StreamingPuller, _ string) ([]byte, error) {
			buffer, err := sp.BugreportToBuffer()
			if err != nil {
				return nil, err
			}
			return buffer.Bytes(), nil
		}},
		{"bugreport-writer", true, func(sp *StreamingPuller, _ string) ([]byte, error) {
			var buffer bytes.Buffer
			err := sp.BugreportToWriter(&buffer)
			return buffer.Bytes(), err
		}},
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content := []byte{'P', 'K', 3, 4, 0, '\r', '\n', 0xff, 0x80}
	for _, method := range methods {
		for _, serial := range []string{"", "serial-1"} {
			for _, name := range []string{"bugreport.zip", "user's files; $(touch marker) & [x].zip", "-option.zip"} {
				t.Run(method.name+"/"+serial+"/"+name, func(t *testing.T) {
					dir := t.TempDir()
					path := filepath.Join(dir, name)
					if name == "-option.zip" {
						t.Chdir(dir)
						path = name
					}
					if err := os.WriteFile(path, content, 0o600); err != nil {
						t.Fatal(err)
					}
					cleanup := filepath.Join(dir, "cleanup")
					t.Setenv("ANDROIDQF_STREAMING_ADB_HELPER", "1")
					t.Setenv("ANDROIDQF_STREAMING_ADB_SERIAL", serial)
					t.Setenv("ANDROIDQF_STREAMING_ADB_PATH", path)
					t.Setenv("ANDROIDQF_STREAMING_ADB_CLEANUP", cleanup)
					puller := NewStreamingPuller(executable, serial, 1)
					got, err := method.pull(puller, path)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, content) {
						t.Fatalf("pulled bytes = %x, want %x", got, content)
					}
					if method.bugreport {
						if _, err := os.Stat(cleanup); err != nil {
							t.Fatalf("bugreport cleanup did not complete: %v", err)
						}
					}
				})
			}
		}
	}
}
