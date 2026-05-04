package ui

import "github.com/sanurb/.dotfiles/apps/cli/internal/state"

// Ports — the UI defines what it needs from the outside world. Adapters
// (in the main package) implement these. The UI compiles and tests
// without ever knowing about Moon, Nix, os/exec, or the filesystem.
//
// This is the only structural concession to ports/adapters in this
// codebase: it exists because the UI must be deterministically testable
// and because the alternative (UI calls main package directly) creates
// an import cycle.

// Mode discriminates the wizard's flow. Realization is never run from
// inside the wizard regardless of mode (ADR-0009).
type Mode int

const (
	ModeInstall    Mode = iota // pillars → scan → snapshot? → realize prompt
	ModeSync                   // skip pillars → scan → snapshot? → auto-yes
	ModeStandalone             // no workspace: pillars → persist; never realizes
)

// Collision is a path under $HOME that exists as a real file or dir
// (not a symlink) and would conflict with a Home Manager projection.
type Collision struct {
	Path string // relative to $HOME, e.g. ".config/ghostty/config"
	Kind string // "file" or "dir"
}

// SnapshotResult is what a successful Snapshot reports back to the UI.
type SnapshotResult struct {
	Count int
	Path  string // absolute backup directory
}

// MoonDeployTask is the Moon target invoked by `dots deploy` and echoed
// to the user during the wizard. Single source of truth: a project-ID
// rename touches one line, not nine.
const MoonDeployTask = "modules:deploy"

// MoonRunDeploy is the rendered command-echo per DESIGN.md.
const MoonRunDeploy = "moon run " + MoonDeployTask

// Snapshotter takes "Safe Snapshots" — quarantines colliding paths.
type Snapshotter interface {
	Scan() ([]Collision, error)
	Snapshot([]Collision) (SnapshotResult, error)
}

// StatePersister snapshots the existing state file (if any) into the
// dots backup directory, then atomically writes the new persona to the
// workspace root. Implementations should fail loudly: a half-written
// state file would feed home.nix garbage.
type StatePersister interface {
	SaveState(state.State) error
}

// Deps is the bag of ports the wizard needs.
//
// Realization is intentionally NOT a port: PR #2 moved `nh home switch`
// out of the wizard entirely. The wizard signals consent via
// Result.RealizeRequested and exits; main.go invokes `dots deploy` as
// a subprocess. ADR-0009 records the rationale.
type Deps struct {
	Snapshotter    Snapshotter
	StatePersister StatePersister

	// Initial is the state read off disk before the wizard launches; the
	// pillar form is pre-seeded with these values so re-running `dots
	// install` shows the user's existing persona, not a blank slate.
	// On first run (no state file) this is state.Default().
	Initial state.State
}

// Option is one row in any of the wizard's selection forms. Value is
// the canonical id persisted to the TOML state file and consumed by
// home.nix; Label/Description are the human-facing text. Pillars and
// extras share this shape because there's nothing meaningfully
// different about them at the TUI layer — only the form widget
// (Select vs MultiSelect) differs.
type Option struct {
	Value       string
	Label       string
	Description string
}

// ShellOptions / TerminalOptions / MultiplexerOptions / CapabilityOptions —
// the closed sets the TUI offers. Adding a new option here without a
// matching nix module under modules/home/{shells,terminals,multiplexers}/ is
// a code smell — home.nix may silently fall through to a default.
var (
	ShellOptions = []Option{
		{Value: "fish", Label: "Fish", Description: "interactive-first, batteries included"},
		{Value: "zsh", Label: "Zsh + Powerlevel10k", Description: "zsh with the p10k prompt (suppresses starship for zsh)"},
		{Value: "nushell", Label: "Nushell", Description: "structured-data shell, modern pipelines"},
	}

	TerminalOptions = []Option{
		{Value: "ghostty", Label: "Ghostty", Description: "GPU-accelerated, native macOS feel"},
		{Value: "kitty", Label: "Kitty", Description: "fast, image protocol, kitten plugins"},
		{Value: "wezterm", Label: "WezTerm", Description: "Lua-configurable, Rust"},
		{Value: "alacritty", Label: "Alacritty", Description: "minimalist, GPU, no tabs"},
	}

	MultiplexerOptions = []Option{
		{Value: "zellij", Label: "Zellij", Description: "layout-driven, discoverable bindings"},
		{Value: "tmux", Label: "Tmux", Description: "battle-tested, scriptable"},
		{Value: "none", Label: "None", Description: "skip the multiplexer entirely"},
	}
)
