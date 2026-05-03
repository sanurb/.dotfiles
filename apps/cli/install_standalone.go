package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
)

// runStandaloneInstall is the workspace-free install path. It runs the
// pillar/capabilities form and writes state to the user-config dir —
// no scan, no snapshot, no realize, since none of those are meaningful
// without the flake. The full wizard's adapters and step machine are
// untouched: this lives entirely in the main package and re-uses the
// already-public Option lists from internal/ui (no port changes).
//
// Trade-off: a second use site for the pillar form definition. If a
// pillar option set changes, both the wizard's capabilitiesForm and
// this function need updating. The Option lists themselves (the
// content) live in one place — internal/ui/deps.go — so the only
// duplication is the four huh widgets below.
func runStandaloneInstall() int {
	path, err := userConfigStatePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}

	initial := loadStandaloneInitial(path)
	formShell := initial.Pillars.Shell
	formTerminal := initial.Pillars.Terminal
	formMultiplexer := initial.Pillars.Multiplexer
	formExtras := extrasFromState(initial.Capabilities)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Shell").
				Options(huhOptions(ui.ShellOptions)...).Value(&formShell),
			huh.NewSelect[string]().Title("Terminal").
				Options(huhOptions(ui.TerminalOptions)...).Value(&formTerminal),
			huh.NewSelect[string]().Title("Multiplexer").
				Options(huhOptions(ui.MultiplexerOptions)...).Value(&formMultiplexer),
			huh.NewMultiSelect[string]().Title("Capabilities").
				Options(huhOptions(ui.CapabilityOptions)...).Value(&formExtras),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return 130
		}
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}

	out := state.State{
		SchemaVersion: state.SchemaVersion,
		Pillars: state.Pillars{
			Shell:       formShell,
			Terminal:    formTerminal,
			Multiplexer: formMultiplexer,
		},
		Capabilities: state.Capabilities{
			Editor: state.Contains(formExtras, "editor"),
			Git:    state.Contains(formExtras, "git"),
		},
	}

	if err := state.Save(path, out); err != nil {
		fmt.Fprintln(os.Stderr, "install: save state:", err)
		return 1
	}
	return 0
}

// userConfigStatePath is the standalone-install state location. Same
// FileName as the workspace-resident copy so home.nix reading either
// path sees the same schema, but kept under XDG_CONFIG_HOME (or its
// fallback) so a user without the workspace has a writable home for
// the profile.
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

// loadStandaloneInitial pre-seeds the form from the user-config copy if
// one exists. Missing / unparseable file → defaults; never an error,
// because the user's path forward is filling in the form anyway.
func loadStandaloneInitial(path string) state.State {
	s, _, err := state.Load(path)
	if err != nil {
		return state.Default()
	}
	return s
}

func extrasFromState(c state.Capabilities) []string {
	out := []string{}
	if c.Editor {
		out = append(out, "editor")
	}
	if c.Git {
		out = append(out, "git")
	}
	return out
}

func huhOptions(opts []ui.Option) []huh.Option[string] {
	out := make([]huh.Option[string], len(opts))
	for i, o := range opts {
		out[i] = huh.NewOption(fmt.Sprintf("%s — %s", o.Label, o.Description), o.Value)
	}
	return out
}
