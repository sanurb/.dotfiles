package ui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// Result is what Run returns. Code follows the dots exit-code convention
// (0 success, 1 internal failure, 130 user abort). RealizeRequested
// signals "user opted in on the Realize-now prompt" — main.go invokes
// `dots deploy` as a subprocess on that signal (ADR-0009).
type Result struct {
	Code             int
	RealizeRequested bool
}

// Run launches the wizard. Callers (main.go) translate Result into
// either a direct os.Exit or a follow-on subprocess invocation.
func Run(mode Mode, deps Deps) Result {
	if mode != ModeStandalone {
		if deps.Snapshotter == nil || deps.StatePersister == nil {
			fmt.Println(theme.Error.Render("✗ wizard misconfigured: missing adapters"))
			return Result{Code: 1}
		}
	}

	prog := tea.NewProgram(New(mode, deps), tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		fmt.Println(theme.Error.Render("✗ tui error: " + err.Error()))
		return Result{Code: 1}
	}

	m, ok := final.(Model)
	if !ok {
		if pm, ok := final.(*Model); ok {
			m = *pm
		} else {
			return Result{Code: 1}
		}
	}

	switch m.step {
	case stepDone:
		return Result{Code: 0, RealizeRequested: m.realizeRequested}
	case stepAborted:
		return Result{Code: 130}
	case stepFailed:
		if m.err == nil {
			m.err = errors.New("unspecified failure")
		}
		fmt.Println(theme.Error.Render("✗ " + m.err.Error()))
		return Result{Code: 1}
	}
	return Result{Code: 1}
}
