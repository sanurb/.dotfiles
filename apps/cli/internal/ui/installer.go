package ui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// Run launches the wizard. The returned exit code matches the dots
// CLI convention: 0 on Done, 1 on Failed, 130 on Aborted (SIGINT-ish).
// Callers (main.go) translate this to os.Exit.
func Run(mode Mode, deps Deps) int {
	if mode != ModeStandalone {
		if deps.Snapshotter == nil || deps.Realizer == nil || deps.StatePersister == nil {
			fmt.Println(theme.Error.Render("✗ wizard misconfigured: missing adapters"))
			return 1
		}
	}

	prog := tea.NewProgram(New(mode, deps), tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		fmt.Println(theme.Error.Render("✗ tui error: " + err.Error()))
		return 1
	}

	m, ok := final.(Model)
	if !ok {
		if pm, ok := final.(*Model); ok {
			m = *pm
		} else {
			return 1
		}
	}

	switch m.step {
	case stepDone:
		return 0
	case stepAborted:
		return 130
	case stepFailed:
		if m.err == nil {
			m.err = errors.New("unspecified failure")
		}
		fmt.Println(theme.Error.Render("✗ " + m.err.Error()))
		return 1
	}
	return 1
}
