package acquisition

import "testing"

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
