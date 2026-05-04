package applied

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	in := State{
		SchemaVersion: SchemaVersion,
		PlanHash:      "abc123",
		Profile:       "work-mac",
		AppliedAt:     time.Date(2026, 5, 3, 18, 30, 0, 0, time.UTC),
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, found, err := Load(path)
	if err != nil || !found {
		t.Fatalf("load: found=%v err=%v", found, err)
	}
	if out.PlanHash != in.PlanHash || out.Profile != in.Profile {
		t.Errorf("round-trip mismatch: in=%+v out=%+v", in, out)
	}
	if !out.AppliedAt.Equal(in.AppliedAt) {
		t.Errorf("AppliedAt mismatch: %v vs %v", in.AppliedAt, out.AppliedAt)
	}
}

func TestLoadMissingIsNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.toml")
	s, found, err := Load(path)
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if found {
		t.Errorf("missing file should report found=false")
	}
	if s.PlanHash != "" {
		t.Errorf("missing file should yield zero state, got %+v", s)
	}
}

func TestSaveDefaultsSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := Save(path, State{PlanHash: "x", AppliedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, _, _ := Load(path)
	if out.SchemaVersion != SchemaVersion {
		t.Errorf("save should default SchemaVersion to %d, got %d", SchemaVersion, out.SchemaVersion)
	}
}
