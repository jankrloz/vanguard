package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryAppendAndRead(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vanguard-hist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	_ = os.Setenv("HOME", tmpDir)

	e1 := Entry{
		Timestamp:   time.Now().Add(-1 * time.Minute),
		PackageID:   "npm:test",
		Manager:     "npm",
		FromVersion: "1.0.0",
		ToVersion:   "1.0.1",
		Status:      "ok",
	}

	e2 := Entry{
		Timestamp:   time.Now(),
		PackageID:   "brew:curl",
		Manager:     "brew",
		FromVersion: "8.0.0",
		ToVersion:   "8.1.0",
		Status:      "ok",
	}

	if err := Append(e1); err != nil {
		t.Fatalf("Append e1: %v", err)
	}
	if err := Append(e2); err != nil {
		t.Fatalf("Append e2: %v", err)
	}

	recent, err := ReadRecent(10)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}

	if len(recent) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(recent))
	}

	// First should be newest (e2)
	if recent[0].PackageID != "brew:curl" {
		t.Errorf("expected e2 first, got %s", recent[0].PackageID)
	}
	_ = filepath.Join
}
