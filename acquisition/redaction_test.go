package acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvt-project/androidqf/adb"
)

func TestRedactHostPath(t *testing.T) {
	for _, tc := range []struct{ home, input, want string }{
		{"/home/alice", "/home/alice/output.zip", "[HOST_HOME]/output.zip"},
		{"/Users/alice", "/Users/alice/.android/adbkey.pub", "[HOST_HOME]/.android/adbkey.pub"},
		{`C:\Users\alice`, `C:\Users\alice\output.zip`, `[HOST_HOME]\output.zip`},
		{`C:\Users\alice`, `C:/Users/alice/output.zip`, `[HOST_HOME]/output.zip`},
		{"/home/alice", "/home/alice-other/output.zip", "/home/alice-other/output.zip"},
		{"/home/alice", "/data/user/0/alice alice.example", "/data/user/0/alice alice.example"},
		{"/home/alice", "/home/alice", "[HOST_HOME]"},
		{"/home/alice", `open "/home/alice": denied`, `open "[HOST_HOME]": denied`},
		{`C:\Users\alice`, `open "C:\\Users\\alice\\output": denied`, `open "[HOST_HOME]\\output": denied`},
		{`C:\Users\Alice`, `c:\users\alice\output`, `[HOST_HOME]\output`},
		{"", "/home/alice/output.zip", "/home/alice/output.zip"},
	} {
		if got := redactHostPath(tc.input, tc.home); got != tc.want {
			t.Errorf("redactHostPath(%q, %q) = %q, want %q", tc.input, tc.home, got, tc.want)
		}
	}
}

func TestCompleteHostRedactionOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "default"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			writer, err := NewStreamingZipWriter("redaction", dir)
			if err != nil {
				t.Fatal(err)
			}
			acq := &Acquisition{
				HostUsernameRedacted: enabled,
				RedactHostHome:       "/home/alice",
				StoragePath:          "/home/alice/output.zip",
				ADBHostPublicKey:     "AAAA-key alice@host",
				TmpDir:               "/data/local/tmp/alice/",
				Collector:            &adb.Collector{Adb: &adb.ADB{ExePath: "/home/alice/bin/adb"}},
				ZipWriter:            writer,
				logBuffer:            bytes.NewBufferString("pull /data/alice to /home/alice/output\n"),
				ModuleResults:        []ModuleResult{{Error: "open /home/alice/output: denied"}},
			}
			if err := writer.CreateFileFromString("device.txt", "alice /home/alice/device evidence"); err != nil {
				t.Fatal(err)
			}
			if err := acq.Complete(); err != nil {
				t.Fatal(err)
			}
			files := readZipFiles(t, writer.GetOutputPath())
			if files["device.txt"] != "alice /home/alice/device evidence" {
				t.Fatal("device evidence changed")
			}
			var info Acquisition
			if err := json.Unmarshal([]byte(files["acquisition.json"]), &info); err != nil {
				t.Fatal(err)
			}
			if info.TmpDir != acq.TmpDir {
				t.Fatal("device metadata changed")
			}
			if acq.StoragePath != "/home/alice/output.zip" || acq.Collector.Adb.ExePath != "/home/alice/bin/adb" {
				t.Fatal("runtime paths changed")
			}
			if enabled {
				if strings.Contains(files["acquisition.json"], "/home/alice") || strings.Contains(files["command.log"], "/home/alice") {
					t.Fatal("host home leaked")
				}
				if files["adb_host_key.pub"] != "AAAA-key\n" || info.ADBHostPublicKey != "AAAA-key" {
					t.Fatal("key comment not removed")
				}
				if !info.HostUsernameRedacted {
					t.Fatal("redaction not recorded")
				}
			} else {
				if info.StoragePath != acq.StoragePath || files["command.log"] != acq.logBuffer.String() || files["adb_host_key.pub"] != acq.ADBHostPublicKey+"\n" {
					t.Fatal("default behavior changed")
				}
			}
			records, err := csv.NewReader(strings.NewReader(files["hashes.csv"])).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			for _, record := range records {
				digest := sha256.Sum256([]byte(files[record[0]]))
				if record[1] != hex.EncodeToString(digest[:]) {
					t.Fatalf("wrong digest for %s", record[0])
				}
			}
		})
	}
}
