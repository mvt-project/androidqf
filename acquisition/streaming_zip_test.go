package acquisition

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestCreateHashListTracksPlaintextZipEntries(t *testing.T) {
	var archive bytes.Buffer
	ezw := &StreamingZipWriter{
		zipWriter: zip.NewWriter(&archive),
	}

	if err := ezw.CreateFileFromString("first.txt", "first content"); err != nil {
		t.Fatalf("CreateFileFromString() error = %v", err)
	}

	writer, err := ezw.CreateFile("stream.bin")
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if _, err := writer.Write([]byte("streamed ")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := writer.Write([]byte("content")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := ezw.CreateHashList(); err != nil {
		t.Fatalf("CreateHashList() error = %v", err)
	}
	if err := ezw.zipWriter.Close(); err != nil {
		t.Fatalf("zip Close() error = %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}

	files := make(map[string]string)
	for _, file := range reader.File {
		readCloser, err := file.Open()
		if err != nil {
			t.Fatalf("Open(%q) error = %v", file.Name, err)
		}
		content, err := io.ReadAll(readCloser)
		readCloser.Close()
		if err != nil {
			t.Fatalf("ReadAll(%q) error = %v", file.Name, err)
		}
		files[file.Name] = string(content)
	}

	records, err := csv.NewReader(strings.NewReader(files["hashes.csv"])).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll(hashes.csv) error = %v", err)
	}

	gotHashes := make(map[string]string)
	for _, record := range records {
		if len(record) != 2 {
			t.Fatalf("hash record has %d fields, want 2: %#v", len(record), record)
		}
		gotHashes[record[0]] = record[1]
	}

	wantHashes := map[string]string{
		"first.txt":  sha256Hex("first content"),
		"stream.bin": sha256Hex("streamed content"),
	}
	if len(gotHashes) != len(wantHashes) {
		t.Fatalf("got %d hash records, want %d: %#v", len(gotHashes), len(wantHashes), gotHashes)
	}
	for name, wantHash := range wantHashes {
		if gotHashes[name] != wantHash {
			t.Fatalf("hash for %q = %q, want %q", name, gotHashes[name], wantHash)
		}
	}
	if _, ok := gotHashes["hashes.csv"]; ok {
		t.Fatal("hashes.csv should not include a hash record for itself")
	}
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func writeTestAgeKey(t *testing.T, dir string) {
	t.Helper()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}

	keyPath := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyPath, []byte(identity.Recipient().String()), 0o600); err != nil {
		t.Fatalf("WriteFile(key.txt) error = %v", err)
	}
}

func TestNewStreamingZipWriterUsesCurrentWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	writeTestAgeKey(t, cwd)

	ezw, err := NewStreamingZipWriter("test-acquisition", cwd)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	defer os.Remove(ezw.GetOutputPath())

	if !ezw.IsEncrypted() {
		t.Fatal("writer is not encrypted with key.txt present")
	}
	if err := ezw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	wantPath := filepath.Join(cwd, "test-acquisition.zip.age")
	if ezw.GetOutputPath() != wantPath {
		t.Fatalf("output path = %q, want %q", ezw.GetOutputPath(), wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("Stat(output) error = %v", err)
	}
}

func TestNewStreamingZipWriterUsesRestrictivePermissions(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "output")

	ezw, err := NewStreamingZipWriter("test-acquisition", outputDir)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}

	dirInfo, err := os.Stat(outputDir)
	if err != nil {
		t.Fatalf("Stat(output directory) error = %v", err)
	}
	if got := dirInfo.Mode().Perm(); got&0o077 != 0 {
		t.Fatalf("output directory permissions = %o, want no group or world permissions", got)
	}

	fileInfo, err := os.Stat(ezw.GetOutputPath())
	if err != nil {
		t.Fatalf("Stat(output file) error = %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("output file permissions during acquisition = %o, want 600", got)
	}

	if err := ezw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	fileInfo, err = os.Stat(ezw.GetOutputPath())
	if err != nil {
		t.Fatalf("Stat(closed output file) error = %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o400 {
		t.Fatalf("closed output file permissions = %o, want 400", got)
	}
}

func TestNewStreamingZipWriterDoesNotOverwriteExistingOutput(t *testing.T) {
	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "test-acquisition.zip")
	original := []byte("existing evidence")
	if err := os.WriteFile(outputPath, original, 0o600); err != nil {
		t.Fatalf("WriteFile(existing output) error = %v", err)
	}

	if _, err := NewStreamingZipWriter("test-acquisition", outputDir); err == nil {
		t.Fatal("NewStreamingZipWriter() returned nil error for existing output")
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(existing output) error = %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("existing output content = %q, want %q", got, original)
	}
}

func TestNewStreamingZipWriterEncryptsForEveryRecipient(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)

	var identities []*age.X25519Identity
	var recipientFile strings.Builder
	recipientFile.WriteString("# Acquisition recipients\n\n")
	for range 2 {
		identity, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatalf("GenerateX25519Identity() error = %v", err)
		}
		identities = append(identities, identity)
		recipientFile.WriteString(identity.Recipient().String())
		recipientFile.WriteByte('\n')
	}
	if err := os.WriteFile(
		filepath.Join(cwd, keyFileName),
		[]byte(recipientFile.String()),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile(key.txt) error = %v", err)
	}

	ezw, err := NewStreamingZipWriter("test-acquisition", cwd)
	if err != nil {
		t.Fatalf("NewStreamingZipWriter() error = %v", err)
	}
	if err := ezw.CreateFileFromString("evidence.txt", "collected evidence"); err != nil {
		t.Fatalf("CreateFileFromString() error = %v", err)
	}
	if err := ezw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	encrypted, err := os.ReadFile(ezw.GetOutputPath())
	if err != nil {
		t.Fatalf("ReadFile(encrypted archive) error = %v", err)
	}
	for i, identity := range identities {
		decrypted, err := age.Decrypt(bytes.NewReader(encrypted), identity)
		if err != nil {
			t.Fatalf("age.Decrypt() for recipient %d error = %v", i+1, err)
		}
		archive, err := io.ReadAll(decrypted)
		if err != nil {
			t.Fatalf("ReadAll(decrypted archive) for recipient %d error = %v", i+1, err)
		}

		reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			t.Fatalf("zip.NewReader() for recipient %d error = %v", i+1, err)
		}
		entry, err := reader.File[0].Open()
		if err != nil {
			t.Fatalf("Open(evidence.txt) for recipient %d error = %v", i+1, err)
		}
		content, err := io.ReadAll(entry)
		entry.Close()
		if err != nil {
			t.Fatalf("ReadAll(evidence.txt) for recipient %d error = %v", i+1, err)
		}
		if string(content) != "collected evidence" {
			t.Fatalf("evidence.txt for recipient %d = %q", i+1, content)
		}
	}
}

func TestValidateZipEntryName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "acquisition.json"},
		{name: "apks/com.example.apk"},
		{name: "logs/data/system/uiderrors.txt"},
		{name: "", wantErr: true},
		{name: "/tmp/evil", wantErr: true},
		{name: "../evil", wantErr: true},
		{name: "apks/../../evil.apk", wantErr: true},
		{name: `..\..\evil`, wantErr: true},
		{name: `apks\..\..\evil.apk`, wantErr: true},
		{name: "C:/evil.apk", wantErr: true},
		{name: "C:evil.apk", wantErr: true},
		{name: "evil\x00.apk", wantErr: true},
	}

	for _, tt := range tests {
		err := validateZipEntryName(tt.name)
		if tt.wantErr && err == nil {
			t.Fatalf("validateZipEntryName(%q) returned nil error", tt.name)
		}
		if !tt.wantErr && err != nil {
			t.Fatalf("validateZipEntryName(%q) returned unexpected error: %v", tt.name, err)
		}
	}
}
