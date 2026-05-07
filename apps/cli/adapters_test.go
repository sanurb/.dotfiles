package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadOverrideStateMissingFileIsError pins the asymmetry between
// the canonical state read (silently falls back to Default when
// absent) and an explicit --config override (must exist). A typo in
// --config should never silently downgrade to Default — it should
// surface as a misuse the user can correct.
func TestLoadOverrideStateMissingFileIsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.toml")
	_, err := loadOverrideState(missing)
	if err == nil {
		t.Fatalf("expected error for missing --config file, got nil")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("error %q should mention 'file not found'", err)
	}
}

// TestLoadOverrideStateRejectsInvalidPersona pins that --config is
// validated, not just parsed. An attacker (or a hand-edit) can't
// smuggle e.g. shell="rm -rf /" through --config because Validate()
// is gated on the closed pillar set.
func TestLoadOverrideStateRejectsInvalidPersona(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	body := `schema_version = 1

[pillars]
shell       = "bogus"
terminal    = "ghostty"
multiplexer = "none"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := loadOverrideState(path)
	if err == nil {
		t.Fatalf("expected validation error for invalid shell, got nil")
	}
	if !strings.Contains(err.Error(), "invalid shell") {
		t.Fatalf("error %q should mention 'invalid shell'", err)
	}
}

// TestLoadOverrideStateLoadsValidPersona is the happy path: a
// well-formed file at the override location yields a valid State.
func TestLoadOverrideStateLoadsValidPersona(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.toml")
	body := `schema_version = 1

[pillars]
shell       = "fish"
terminal    = "kitty"
multiplexer = "tmux"

[capabilities]
editor = false
font   = true
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s, err := loadOverrideState(path)
	if err != nil {
		t.Fatalf("loadOverrideState: %v", err)
	}
	if s.Pillars.Shell != "fish" || s.Pillars.Terminal != "kitty" || s.Pillars.Multiplexer != "tmux" {
		t.Fatalf("pillars not loaded: got %+v", s.Pillars)
	}
	if s.Capabilities.Editor || !s.Capabilities.Font {
		t.Fatalf("capabilities not loaded: got %+v", s.Capabilities)
	}
}
