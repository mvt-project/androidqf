//go:build !unbundle

package assets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDeployAssetsWritesEveryEmbeddedAsset(t *testing.T) {
	if err := CleanAssets(); err != nil {
		t.Fatalf("CleanAssets() error = %v", err)
	}
	t.Cleanup(func() {
		if err := CleanAssets(); err != nil {
			t.Errorf("CleanAssets() error = %v", err)
		}
	})

	dir, err := DeployAssets()
	if err != nil {
		t.Fatalf("DeployAssets() error = %v", err)
	}
	for _, asset := range getAssets() {
		path := filepath.Join(dir, asset.Name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		if !bytes.Equal(data, asset.Data) {
			t.Fatalf("deployed asset %q does not match embedded data", asset.Name)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("deployed asset %q is not executable: %v", asset.Name, info.Mode())
		}
	}
}
