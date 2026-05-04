package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
)

// runStandaloneInstall captures a profile to ~/.config/dots/ when no
// workspace is reachable. Reuses the bubbletea wizard in ModeStandalone,
// which short-circuits Done after persisting — no scan/snapshot/realize.
func runStandaloneInstall() int {
	path, err := userConfigStatePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}

	deps := ui.Deps{
		StatePersister: standalonePersister{path: path},
		Initial:        loadStandaloneInitial(path),
	}
	// Standalone never realizes — there is no workspace — so we
	// propagate Result.Code only; RealizeRequested is structurally
	// false on this path.
	return ui.Run(ui.ModeStandalone, deps).Code
}

// standalonePersister implements ui.StatePersister against a user-config
// path (typically ~/.config/dots/.dots-state.toml). No backup snapshot
// is taken — without a workspace there is no .dots_backups/<ts>/ folder
// convention to honor, and a simple atomic Save handles partial-write
// safety.
type standalonePersister struct {
	path string
}

func (p standalonePersister) SaveState(s state.State) error {
	return state.Save(p.path, s)
}

func userConfigStatePath() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "dots", state.FileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, ".config", "dots", state.FileName), nil
}

func loadStandaloneInitial(path string) state.State {
	if s, _, err := state.Load(path); err == nil {
		return s
	}
	return state.Default()
}
