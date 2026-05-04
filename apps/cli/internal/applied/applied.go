// Package applied owns the on-disk record of "what dots last applied
// to this host." It is the readable receipt for `dots status`: the
// hash of the last-applied plan, the profile, and the timestamp.
//
// The file lives at $DOTS_STATE_HOME or $XDG_STATE_HOME/dots/applied.toml,
// falling back to ~/.local/state/dots/applied.toml. It is deliberately
// distinct from the workspace's .dots-state.toml — that file is the
// declarative *input* (persona/profile selection consumed by home.nix);
// this file is the procedural *output* of the most recent apply.
package applied

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SchemaVersion bumps when the file shape changes incompatibly.
const SchemaVersion = 1

// FileName is the canonical file name under the state directory.
const FileName = "applied.toml"

// State is the parsed contents of applied.toml. SchemaVersion is
// validated on Load; other fields are best-effort and may be empty
// on a freshly-installed dots.
type State struct {
	SchemaVersion int
	PlanHash      string
	Profile       string
	AppliedAt     time.Time
}

// DefaultPath resolves the applied-state file path per XDG-with-DOTS-overrides.
func DefaultPath() (string, error) {
	if v := os.Getenv("DOTS_STATE_HOME"); v != "" {
		return filepath.Join(v, "dots", FileName), nil
	}
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, "dots", FileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".local", "state", "dots", FileName), nil
}

// Load reads the applied-state file. Missing file returns (zero,
// false, nil) — it is the legitimate first-run condition. A parse
// error returns it to the caller; status surfaces it but does not
// block other verbs.
func Load(path string) (State, bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	defer f.Close()

	s := State{SchemaVersion: SchemaVersion}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		switch key {
		case "schema_version":
			fmt.Sscanf(val, "%d", &s.SchemaVersion)
		case "plan_hash":
			s.PlanHash = trimQuotes(val)
		case "profile":
			s.Profile = trimQuotes(val)
		case "applied_at":
			t, err := time.Parse(time.RFC3339, trimQuotes(val))
			if err == nil {
				s.AppliedAt = t
			}
		}
	}
	return s, true, sc.Err()
}

// Save writes atomically: stage to .tmp then rename. Caller is
// responsible for invocation timing — typically the last successful
// step of `dots apply`.
func Save(path string, s State) error {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f,
		"# applied.toml — managed by `dots apply`. Edits are overwritten.\nschema_version = %d\nplan_hash = %q\nprofile = %q\napplied_at = %q\n",
		s.SchemaVersion, s.PlanHash, s.Profile, s.AppliedAt.Format(time.RFC3339)); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func trimQuotes(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}
