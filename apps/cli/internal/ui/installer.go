package ui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// Result is what Run returns to the dispatcher in main.go.
//
// Code maps onto the dots CLI exit-code convention: 0 on the user's
// success path, 1 on a wizard-internal failure, 130 on user-aborted
// (SIGINT-ish).
//
// RealizeRequested signals to main that the user picked "Yes" on the
// post-confirm "Realize now?" prompt. main is responsible for invoking
// `dots deploy` as a subprocess; the wizard never realizes the system
// itself. See ADR-0009 for the rationale.
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
