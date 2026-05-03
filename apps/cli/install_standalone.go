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

// runStandaloneInstall captures a profile to ~/.config/dots/ when no
// workspace is reachable. Skips scan/snapshot/realize since those need
// the flake. Reuses ui.HuhOptions and Capabilities.Extras() so the
// form here and the wizard's capabilitiesForm stay aligned.
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
	formExtras := initial.Capabilities.Extras()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Shell").
				Options(ui.HuhOptions(ui.ShellOptions, 0)...).Value(&formShell),
			huh.NewSelect[string]().Title("Terminal").
				Options(ui.HuhOptions(ui.TerminalOptions, 0)...).Value(&formTerminal),
			huh.NewSelect[string]().Title("Multiplexer").
				Options(ui.HuhOptions(ui.MultiplexerOptions, 0)...).Value(&formMultiplexer),
			huh.NewMultiSelect[string]().Title("Capabilities").
				Options(ui.HuhOptions(ui.CapabilityOptions, 0)...).Value(&formExtras),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			// 130 = 128 + SIGINT, matching shell convention for Ctrl+C —
			// also what ui.Run returns for stepAborted.
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
// FileName as the workspace-resident copy so the schema is identical,
// just kept under XDG_CONFIG_HOME so a user without the workspace has
// a writable home for the profile.
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
