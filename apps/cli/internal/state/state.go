// Package state owns the on-disk persona file (.dots-state.toml) that
// declares which shell, terminal, and multiplexer this host runs. It is
// the single source of truth shared between the dots TUI (writer), the
// home.nix module (reader, via builtins.fromTOML), and the doctor command
// (auditor).
//
// We hand-roll TOML for our flat schema (3 strings + a small set of
// bools) rather than pulling in a dependency. The schema is closed and
// small; a ~80-line parser is preferable to a third-party module that
// the doctor would then also have to verify.
package state

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SCHEMA_VERSION is the single source of truth shared between Go (this
// package, via go:embed) and Nix (modules/profiles/home.nix, via
// builtins.readFile). Bumping requires editing exactly one file; both
// sides pick the new value up automatically. Drift between Go and Nix
// is impossible by construction.
//
//go:embed SCHEMA_VERSION
var schemaVersionRaw string

// FileName is the canonical state file name, written at the workspace root.
const FileName = ".dots-state.toml"

// SchemaVersion bumps when the file shape changes incompatibly.
var SchemaVersion = func() int {
	n, err := strconv.Atoi(strings.TrimSpace(schemaVersionRaw))
	if err != nil {
		panic("state: SCHEMA_VERSION must be an integer, got " + strconv.Quote(schemaVersionRaw))
	}
	return n
}()

type State struct {
	SchemaVersion int
	Pillars       Pillars
	Capabilities  Capabilities
	Modules       Modules
}

type Pillars struct {
	Shell       string // "fish" | "zsh" | "nushell"
	Terminal    string // "ghostty" | "kitty" | "wezterm" | "alacritty"
	Multiplexer string // "zellij" | "tmux" | "none"
}

// Format renders the persona one-liner used by `dots status`,
// `dots profile`, and the wizard's done-summary. Single source so the
// three surfaces never drift.
func (p Pillars) Format() string {
	return p.Shell + " · " + p.Terminal + " · " + p.Multiplexer
}

// Capabilities are user-facing feature categories that toggle on or
// off. They map to feature-class modules (editor.nix, font.nix), each
// of which encapsulates a curated set of packages and configuration.
// Distinct from Modules below, which toggles concrete packages.
type Capabilities struct {
	Editor bool
	Font   bool
}

// Modules toggles concrete satellite packages individually. Foundation
// (atuin/zoxide/starship/git/nix-index) and pillars (shell/terminal/
// multiplexer) are out of scope here — those are mandatory or
// mutually exclusive choices; Modules is the opt-out surface for
// always-default-true convenience tools.
//
// Default is true for every field. The parser explicitly defaults
// absent fields to true so a v1 state file (no [modules] section)
// upgrades to v2 with zero behavior change.
type Modules struct {
	Bat      bool
	Delta    bool
	Gh       bool
	Opencode bool
}

// Skipped returns the lower-case names of satellite modules whose
// install is suppressed in this state. Used by status/profile/doctor
// surfaces that need to render an opt-out summary; centralized here
// so the four call-sites never drift on naming or order.
func (m Modules) Skipped() []string {
	var out []string
	if !m.Bat {
		out = append(out, "bat")
	}
	if !m.Delta {
		out = append(out, "delta")
	}
	if !m.Gh {
		out = append(out, "gh")
	}
	if !m.Opencode {
		out = append(out, "opencode")
	}
	return out
}

// Allowed values per pillar — the TUI offers exactly these and the doctor
// rejects anything outside the set. Foundation tools (atuin, zoxide,
// starship) are intentionally absent: they are not user choices.
var (
	ValidShells       = []string{"fish", "zsh", "nushell"}
	ValidTerminals    = []string{"ghostty", "kitty", "wezterm", "alacritty"}
	ValidMultiplexers = []string{"zellij", "tmux", "none"}
)

// Default returns the opinionated 2026 starting point: behavior-preserving
// for the modules that previously hard-coded fish + ghostty + zellij.
// Every Modules field defaults to true so a fresh host gets the full
// satellite set; opt-out is explicit (TUI write, --config seed, or
// hand-edit).
func Default() State {
	return State{
		SchemaVersion: SchemaVersion,
		Pillars:       Pillars{Shell: "fish", Terminal: "ghostty", Multiplexer: "zellij"},
		Capabilities:  Capabilities{Editor: true, Font: true},
		Modules:       Modules{Bat: true, Delta: true, Gh: true, Opencode: true},
	}
}

// Path returns the absolute path to the state file at the given workspace root.
func Path(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, FileName)
}

// Load reads the state file. If it doesn't exist, returns Default() and
// found=false so callers can distinguish "first run" from "explicit reset".
func Load(path string) (s State, found bool, err error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return Default(), false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	defer f.Close()
	parsed, perr := parse(f)
	if perr != nil {
		return State{}, false, fmt.Errorf("parse %s: %w", path, perr)
	}
	return parsed, true, nil
}

// Save writes atomically: stage to a sibling .tmp file, then rename. The
// .dots-state.toml file is the input to home.nix, so a partial write
// during a power loss would leave Nix evaluation in a half-state.
//
// SchemaVersion is normalized to the current package version on every
// write. Reads preserve whatever the file claimed (so callers can audit
// staleness), but a write is the migration boundary: by the time we
// emit, the on-disk shape *is* the current schema. Skipping this would
// produce a "v1 file with v2 sections" — a structural lie about
// version metadata.
func Save(path string, s State) error {
	s.SchemaVersion = SchemaVersion
	if err := s.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := emit(f, s); err != nil {
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

// Validate enforces that pillar values are within the closed sets above.
// We never want home.nix to receive an unknown pillar and silently fall
// through to a default — that would mask a TUI bug.
func (s State) Validate() error {
	if !Contains(ValidShells, s.Pillars.Shell) {
		return fmt.Errorf("invalid shell %q (allowed: %s)", s.Pillars.Shell, strings.Join(ValidShells, ", "))
	}
	if !Contains(ValidTerminals, s.Pillars.Terminal) {
		return fmt.Errorf("invalid terminal %q (allowed: %s)", s.Pillars.Terminal, strings.Join(ValidTerminals, ", "))
	}
	if !Contains(ValidMultiplexers, s.Pillars.Multiplexer) {
		return fmt.Errorf("invalid multiplexer %q (allowed: %s)", s.Pillars.Multiplexer, strings.Join(ValidMultiplexers, ", "))
	}
	return nil
}

// parse handles our closed schema:
//
//	schema_version = 2
//	[pillars]
//	shell       = "fish"
//	terminal    = "ghostty"
//	multiplexer = "zellij"
//	[capabilities]
//	editor = true
//	font   = true
//	[modules]
//	bat      = true
//	delta    = true
//	gh       = true
//	opencode = true
//
// Anything outside this shape is ignored — we never want a typo'd key to
// surface as a parser error and block a deploy. Validate() catches the
// values that actually matter.
//
// v1 → v2 backcompat: out is seeded from Default() so an absent
// [modules] section (the v1 shape) yields all-satellites-enabled, which
// is behavior-preserving. The schema_version field is updated by the
// next Save(); reads never rewrite the file.
func parse(r io.Reader) (State, error) {
	out := Default()
	sc := bufio.NewScanner(r)
	section := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		switch section {
		case "":
			if key == "schema_version" {
				var n int
				if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
					out.SchemaVersion = n
				}
			}
		case "pillars":
			s, ok := unquote(val)
			if !ok {
				return State{}, fmt.Errorf("pillars.%s: expected quoted string, got %q", key, val)
			}
			switch key {
			case "shell":
				out.Pillars.Shell = s
			case "terminal":
				out.Pillars.Terminal = s
			case "multiplexer":
				out.Pillars.Multiplexer = s
			}
		case "capabilities":
			b, ok := parseBool(val)
			if !ok {
				return State{}, fmt.Errorf("capabilities.%s: expected bool, got %q", key, val)
			}
			switch key {
			case "editor":
				out.Capabilities.Editor = b
			case "font":
				out.Capabilities.Font = b
			}
		case "modules":
			b, ok := parseBool(val)
			if !ok {
				return State{}, fmt.Errorf("modules.%s: expected bool, got %q", key, val)
			}
			switch key {
			case "bat":
				out.Modules.Bat = b
			case "delta":
				out.Modules.Delta = b
			case "gh":
				out.Modules.Gh = b
			case "opencode":
				out.Modules.Opencode = b
			}
		}
	}
	return out, sc.Err()
}

func emit(w io.Writer, s State) error {
	const tmpl = `# .dots-state.toml — managed by the dots CLI. Edit via 'dots install'.
# This file is the single source of truth for the environment's persona:
# shell, terminal, multiplexer. home.nix reads it via builtins.fromTOML.
# Mandatory infrastructure (atuin, zoxide, starship, git, nix-index) is
# not represented here — those are non-negotiable and live in
# modules/home/foundation.nix and modules/home/git.nix.

schema_version = %d

[pillars]
shell       = %q
terminal    = %q
multiplexer = %q

[capabilities]
editor = %v
font   = %v

# Satellite tools — opt out by setting any to false. Defaults are true.
# The home-manager profile imports each module via lib.optional, so a
# false here removes the package from the realized closure on the next
# 'dots apply'.
[modules]
bat      = %v
delta    = %v
gh       = %v
opencode = %v
`
	_, err := fmt.Fprintf(w, tmpl,
		s.SchemaVersion,
		s.Pillars.Shell, s.Pillars.Terminal, s.Pillars.Multiplexer,
		s.Capabilities.Editor, s.Capabilities.Font,
		s.Modules.Bat, s.Modules.Delta, s.Modules.Gh, s.Modules.Opencode)
	return err
}

func unquote(v string) (string, bool) {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1], true
	}
	return "", false
}

func parseBool(v string) (bool, bool) {
	switch v {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// Contains is a small string-slice membership helper, exported so
// the ui package can share the same check rather than maintaining a
// parallel implementation.
func Contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
