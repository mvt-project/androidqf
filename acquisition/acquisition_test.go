package acquisition

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreInfoSetsCompletedTimestamp(t *testing.T) {
	acq := &Acquisition{
		UUID:        "test-acquisition",
		StoragePath: t.TempDir(),
	}

	if err := acq.StoreInfo(); err != nil {
		t.Fatalf("StoreInfo() error = %v", err)
	}

	if acq.Completed.IsZero() {
		t.Fatal("StoreInfo() left Completed unset")
	}

	info, err := os.ReadFile(filepath.Join(acq.StoragePath, "acquisition.json"))
	if err != nil {
		t.Fatalf("ReadFile(acquisition.json) error = %v", err)
	}

	var stored Acquisition
	if err := json.Unmarshal(info, &stored); err != nil {
		t.Fatalf("json.Unmarshal(acquisition.json) error = %v", err)
	}
	if stored.Completed.IsZero() {
		t.Fatal("acquisition.json contains a zero completed timestamp")
	}
}

func TestCompleteDoesNotOverwriteExistingCompletedTimestamp(t *testing.T) {
	acq := &Acquisition{
		UUID:        "test-acquisition",
		StoragePath: t.TempDir(),
	}

	if err := acq.StoreInfo(); err != nil {
		t.Fatalf("StoreInfo() error = %v", err)
	}
	completed := acq.Completed

	acq.Complete()

	if !acq.Completed.Equal(completed) {
		t.Fatalf("Complete() changed Completed from %s to %s", completed, acq.Completed)
	}
}
