package adb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRootShellPreservesCompoundCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executes the POSIX shell used by adb shell")
	}
	dir := t.TempDir()
	// adb shell joins arguments and lets the remote shell parse them. The
	// su fixture requires the entire -c command to survive as one argument.
	for name, script := range map[string]string{
		"adb": "#!/bin/sh\n[ \"$1\" = shell ] || exit 2\nshift\nexec /bin/sh -c \"$*\"\n",
		"su":  "#!/bin/sh\n[ \"$#\" = 2 ] && [ \"$1\" = -c ] || exit 3\nexec /bin/sh -c \"$2\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	client := &ADB{ExePath: filepath.Join(dir, "adb")}
	command := "if [ 1 = 1 ]; then printf '%s' \"quoted's ; value\"; fi"
	got, err := client.RootShell(command)
	if err != nil || got != "quoted's ; value" {
		t.Fatalf("RootShell() = %q, %v; want intact compound-command output", got, err)
	}
}
