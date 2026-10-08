package modules

import (
	"archive/zip"
	"errors"
	"io"
	"testing"

	"github.com/mvt-project/androidqf/acquisition"
)

func TestSELinuxRunCollectsStatusAndAvailablePolicies(t *testing.T) {
	writer := newModuleTestZipWriter(t)
	acq := &acquisition.Acquisition{
		ZipWriter:     writer,
		StreamingMode: true,
	}

	existing := map[string]bool{
		"/odm/etc/selinux/precompiled_sepolicy": true,
	}
	var pulls []selinuxPolicyFile
	err := NewSELinux().run(
		acq,
		func(command ...string) (string, error) {
			if len(command) != 1 || command[0] != "getenforce" {
				t.Fatalf("shell command = %#v, want getenforce", command)
			}
			return "Enforcing", nil
		},
		func(path string) (bool, error) {
			return existing[path], nil
		},
		func(remotePath, archivePath string) error {
			pulls = append(pulls, selinuxPolicyFile{
				remotePath:  remotePath,
				archivePath: archivePath,
			})
			return nil
		},
	)
	if err != nil {
		t.Fatalf("SELinux.run() error = %v", err)
	}

	if len(pulls) != 2 {
		t.Fatalf("policy pulls = %#v, want ODM and active policies", pulls)
	}
	if pulls[0].remotePath != "/odm/etc/selinux/precompiled_sepolicy" ||
		pulls[0].archivePath != "selinux/odm/etc/selinux/precompiled_sepolicy" {
		t.Fatalf("first policy pull = %#v, want ODM policy mapping", pulls[0])
	}
	if pulls[1].remotePath != "/sys/fs/selinux/policy" ||
		pulls[1].archivePath != "selinux/sys/fs/selinux/policy" {
		t.Fatalf("second policy pull = %#v, want active policy mapping", pulls[1])
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	files := readModuleTestZip(t, writer.GetOutputPath())
	if got := files["selinux.txt"]; got != "Enforcing" {
		t.Fatalf("selinux.txt = %q, want Enforcing", got)
	}
}

func TestSELinuxRunAttemptsAllPoliciesAndAggregatesFailures(t *testing.T) {
	writer := newModuleTestZipWriter(t)
	acq := &acquisition.Acquisition{
		ZipWriter:     writer,
		StreamingMode: true,
	}
	t.Cleanup(func() {
		_ = writer.Close()
	})

	statusErr := errors.New("getenforce failed")
	odmErr := errors.New("odm pull failed")
	activeErr := errors.New("active pull failed")
	var pulls []string
	err := NewSELinux().run(
		acq,
		func(...string) (string, error) {
			return "", statusErr
		},
		func(string) (bool, error) {
			return true, nil
		},
		func(remotePath, archivePath string) error {
			pulls = append(pulls, remotePath)
			switch remotePath {
			case "/odm/etc/selinux/precompiled_sepolicy":
				return odmErr
			case "/sys/fs/selinux/policy":
				return activeErr
			default:
				return nil
			}
		},
	)

	if !errors.Is(err, statusErr) || !errors.Is(err, odmErr) || !errors.Is(err, activeErr) {
		t.Fatalf("SELinux.run() error = %v, want all failures", err)
	}
	if len(pulls) != len(selinuxPolicyFiles) {
		t.Fatalf("attempted %d policy pulls, want %d", len(pulls), len(selinuxPolicyFiles))
	}
}

func newModuleTestZipWriter(t *testing.T) *acquisition.StreamingZipWriter {
	t.Helper()

	writer, err := acquisition.NewStreamingZipWriter("selinux-test", t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	return writer
}

func readModuleTestZip(t *testing.T, archivePath string) map[string]string {
	t.Helper()

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("zip.OpenReader() error = %v", err)
	}
	defer reader.Close()

	files := make(map[string]string)
	for _, file := range reader.File {
		fileReader, err := file.Open()
		if err != nil {
			t.Fatalf("Open(%q) error = %v", file.Name, err)
		}
		content, err := io.ReadAll(fileReader)
		_ = fileReader.Close()
		if err != nil {
			t.Fatalf("ReadAll(%q) error = %v", file.Name, err)
		}
		files[file.Name] = string(content)
	}
	return files
}
