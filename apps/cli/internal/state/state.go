// Package state owns the on-disk persona file (.dots-state.toml) that
// declares which shell, terminal, and multiplexer this host runs. It is
// the single source of truth shared between the dots TUI (writer), the
// home.nix module (reader, via builtins.fromTOML), and the doctor command
// (auditor).
//
// We hand-roll TOML for our flat schema (3 strings + 2 bools) rather than
// pulling in a dependency. The schema is closed and small; a 60-line
// parser is preferable to a third-party module that the doctor would then
// also have to verify.
package state

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// FileName is the canonical state file name, written at the workspace root.
	FileName = ".dots-state.toml"

	// SchemaVersion bumps when the file shape changes incompatibly.
	SchemaVersion = 1
)

type State struct {
	SchemaVersion int
	Pillars       Pillars
	Capabilities  Capabilities
}

type Pillars struct {
	Shell       string // "fish" | "zsh" | "nushell"
	Terminal    string // "ghostty" | "kitty" | "wezterm" | "alacritty"
	Multiplexer string // "zellij" | "tmux" | "none"
}

type Capabilities struct {
	Editor bool
	Git    bool
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
func Default() State {
	return State{
		SchemaVersion: SchemaVersion,
		Pillars:       Pillars{Shell: "fish", Terminal: "ghostty", Multiplexer: "zellij"},
		Capabilities:  Capabilities{Editor: true, Git: true},
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
func Save(path string, s State) error {
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
//	schema_version = 1
//	[pillars]
//	shell       = "fish"
//	terminal    = "ghostty"
//	multiplexer = "zellij"
//	[capabilities]
//	editor = true
//	git    = true
//
// Anything outside this shape is ignored — we never want a typo'd key to
// surface as a parser error and block a deploy. Validate() catches the
// values that actually matter.
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
			case "git":
				out.Capabilities.Git = b
			}
		}
	}
	return out, sc.Err()
}

func emit(w io.Writer, s State) error {
	const tmpl = `# .dots-state.toml — managed by the dots CLI. Edit via 'dots install'.
# This file is the single source of truth for the environment's persona:
# shell, terminal, multiplexer. home.nix reads it via builtins.fromTOML.
# Mandatory infrastructure (atuin, zoxide, starship) is not represented
# here — those are non-negotiable and live in modules/foundation.nix.

schema_version = %d

[pillars]
shell       = %q
terminal    = %q
multiplexer = %q

[capabilities]
editor = %v
git    = %v
`
	_, err := fmt.Fprintf(w, tmpl,
		s.SchemaVersion,
		s.Pillars.Shell, s.Pillars.Terminal, s.Pillars.Multiplexer,
		s.Capabilities.Editor, s.Capabilities.Git)
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
