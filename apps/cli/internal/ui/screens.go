package ui

import (
	"fmt"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/components/screen"
	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// layout returns the screen.Layout for the current step. The dispatcher's
// View calls this once per render.
func (m Model) layout() screen.Layout {
	switch m.step {
	case stepWelcome:
		return m.welcomeLayout()
	case stepShell, stepTerminal, stepMultiplexer, stepEditor, stepGit:
		return m.pillarLayout()
	case stepConfirm:
		return m.confirmLayout()
	case stepConflict:
		return m.conflictLayout()
	case stepScanning:
		return m.transientLayout("Scanning $HOME for brownfield collisions…")
	case stepSnapshotting:
		return m.transientLayout("Snapshotting colliding paths into ~/.dots_backups…")
	case stepRealizing:
		return m.realizingLayout()
	case stepDone:
		return m.doneLayout()
	case stepFailed:
		return m.failedLayout()
	case stepAborted:
		return m.abortedLayout()
	}
	return screen.Layout{}
}

// currentRows returns the option rows for the active selection step.
// Used by handleSelectKey to bound cursor movement, and indirectly by
// pillarLayout / welcomeLayout / confirmLayout / conflictLayout to
// render the option list.
func (m Model) currentRows() []screen.Row {
	switch m.step {
	case stepWelcome:
		return optionRows(welcomeOptions(m.mode), m.cursor)
	case stepShell:
		return optionRows(asWizardOptions(ShellOptions), m.cursor)
	case stepTerminal:
		return optionRows(asWizardOptions(TerminalOptions), m.cursor)
	case stepMultiplexer:
		return optionRows(asWizardOptions(MultiplexerOptions), m.cursor)
	case stepEditor:
		return optionRows(editorOptions(), m.cursor)
	case stepGit:
		return optionRows(gitOptions(), m.cursor)
	case stepConfirm:
		return optionRows(confirmOptions(m.mode), m.cursor)
	case stepConflict:
		return optionRows(conflictOptions(), m.cursor)
	}
	return nil
}

// -- welcome --------------------------------------------------------

func (m Model) welcomeLayout() screen.Layout {
	title := "dots install"
	desc := "Realize the workspace's Home Manager state into ~/."
	if m.mode == ModeSync {
		title = "dots sync"
		desc = "Re-realize the workspace's Home Manager state. Persona is preserved."
	}
	if m.mode == ModeStandalone {
		desc = "Capture a profile to ~/.config/dots. Realization requires the workspace clone + Nix."
	}
	return screen.Layout{
		Title:       title,
		Description: desc,
		Content:     screen.RenderRows(m.currentRows()),
		Keymap:      screen.DefaultKeymap(false),
		Width:       m.width,
	}
}

// -- pillar selection (shell / terminal / multiplexer / editor / git) --

func (m Model) pillarLayout() screen.Layout {
	idx := stepperIndex(m.step)
	title, desc := pillarCopy(m.step)
	return screen.Layout{
		Stepper:     &screen.Stepper{Steps: stepperLabels, Current: idx},
		Heading:     fmt.Sprintf("Step %d:", idx+1),
		Title:       title,
		Description: desc,
		Content:     screen.RenderRows(m.currentRows()),
		Keymap:      screen.DefaultKeymap(idx > 0),
		Width:       m.width,
	}
}

// stepperIndex maps a wizard stepID onto its 0-based stepper position.
// Only valid for the six pillar/confirm steps.
func stepperIndex(s stepID) int {
	return int(s - stepShell)
}

func pillarCopy(s stepID) (title, desc string) {
	switch s {
	case stepShell:
		return "Shell", "One shell defines the persona. Atuin, zoxide, and starship are baked in regardless."
	case stepTerminal:
		return "Terminal Emulator", "Pick the terminal app to configure."
	case stepMultiplexer:
		return "Multiplexer", "Pick one, or 'None' to skip."
	case stepEditor:
		return "Neovim", "Includes LSP, TreeSitter, and the dots Neovim config."
	case stepGit:
		return "Git defaults", "Global git config (identity stays external)."
	}
	return "", ""
}

// -- confirm --------------------------------------------------------

func (m Model) confirmLayout() screen.Layout {
	idx := stepperIndex(stepConfirm)
	persona := fmt.Sprintf("%s · %s · %s",
		m.formShell, m.formTerminal, m.formMultiplexer)
	caps := []string{}
	if m.formEditor {
		caps = append(caps, "neovim")
	}
	if m.formGit {
		caps = append(caps, "git")
	}
	capsLine := "(none)"
	if len(caps) > 0 {
		capsLine = strings.Join(caps, ", ")
	}
	desc := fmt.Sprintf("persona: %s   capabilities: %s", persona, capsLine)

	km := screen.DefaultKeymap(true)
	km.SelectVerb = "confirm"

	return screen.Layout{
		Stepper:     &screen.Stepper{Steps: stepperLabels, Current: idx},
		Heading:     fmt.Sprintf("Step %d:", idx+1),
		Title:       "Confirm",
		Description: desc,
		Content:     screen.RenderRows(m.currentRows()),
		Keymap:      km,
		Width:       m.width,
	}
}

// -- conflict -------------------------------------------------------

func (m Model) conflictLayout() screen.Layout {
	var sb strings.Builder
	sb.WriteString(theme.Muted.Render("The following paths exist as real files/dirs and"))
	sb.WriteString("\n")
	sb.WriteString(theme.Muted.Render("would collide with Home Manager symlinks."))
	sb.WriteString("\n\n")
	for _, c := range m.collisions {
		sb.WriteString("  " + theme.Muted.Render(fmt.Sprintf("[%s]", c.Kind)) + " " + theme.Body.Render(c.Path) + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(screen.RenderRows(m.currentRows()))

	km := screen.DefaultKeymap(false)
	km.SelectVerb = "snapshot"

	return screen.Layout{
		Title:       "Brownfield conflicts",
		Description: fmt.Sprintf("%d colliding path(s) detected — safe to snapshot into ~/.dots_backups/<ts>/", len(m.collisions)),
		Content:     sb.String(),
		Keymap:      km,
		Width:       m.width,
	}
}

// -- transient (scan / snapshot / realize) -------------------------

func (m Model) transientLayout(message string) screen.Layout {
	return screen.Layout{
		Title:     "",
		Heading:   "",
		State:     screen.StateLoading,
		StatusMsg: m.spinner.View() + " " + message,
		Keymap:    screen.Keymap{Quit: "Ctrl-C"},
		Width:     m.width,
	}
}

func (m Model) realizingLayout() screen.Layout {
	body := strings.Join([]string{
		fmt.Sprintf("%s %s", m.spinner.View(), theme.Body.Render("Realizing Home Manager state…")),
		"",
		m.progress.View(),
		"",
		theme.Muted.Render(MoonRunDeploy),
	}, "\n")
	return screen.Layout{
		Content: body,
		Keymap:  screen.Keymap{Quit: "Ctrl-C"},
		Width:   m.width,
	}
}

// -- outcome surfaces ----------------------------------------------

func (m Model) doneLayout() screen.Layout {
	return screen.Layout{
		Heading:  "✓",
		Title:    "Realized",
		Content:  m.renderDoneSummary(),
		Bordered: true,
		Keymap:   screen.Keymap{Select: "Enter", SelectVerb: "dismiss"},
		Width:    m.width,
	}
}

func (m Model) failedLayout() screen.Layout {
	msg := "unspecified failure"
	if m.err != nil {
		msg = m.err.Error()
	}
	return screen.Layout{
		Heading:   "✗",
		Title:     "Failed",
		State:     screen.StateError,
		StatusMsg: msg,
		Bordered:  true,
		Keymap:    screen.Keymap{Select: "Enter", SelectVerb: "dismiss"},
		Width:     m.width,
	}
}

func (m Model) abortedLayout() screen.Layout {
	return screen.Layout{
		Title:    "Aborted",
		Bordered: true,
		Keymap:   screen.Keymap{Select: "Enter", SelectVerb: "dismiss"},
		Width:    m.width,
	}
}

// -- option helpers ------------------------------------------------

// wizardOption is the row-shape consumed by the rendering layer; it
// wraps Option (the public TUI input) plus the Yes/No options used by
// boolean-pillar screens (editor, git) and the welcome/confirm/conflict
// screens, none of which appear in the public Option lists.
type wizardOption struct {
	Label  string
	Detail string
}

func asWizardOptions(opts []Option) []wizardOption {
	out := make([]wizardOption, len(opts))
	for i, o := range opts {
		out[i] = wizardOption{Label: o.Label, Detail: o.Description}
	}
	return out
}

func optionRows(opts []wizardOption, cursor int) []screen.Row {
	rows := make([]screen.Row, len(opts))
	for i, o := range opts {
		rows[i] = screen.Row{
			Label:    o.Label,
			Detail:   o.Detail,
			Cursor:   i == cursor,
			Selected: i == cursor,
		}
	}
	return rows
}

func welcomeOptions(mode Mode) []wizardOption {
	verb := "Continue"
	switch mode {
	case ModeSync:
		verb = "Re-realize"
	case ModeStandalone:
		verb = "Capture profile"
	}
	return []wizardOption{
		{Label: verb},
		{Label: "Cancel"},
	}
}

func editorOptions() []wizardOption {
	return []wizardOption{
		{Label: "Yes, install Neovim with config"},
		{Label: "No, skip Neovim"},
	}
}

func gitOptions() []wizardOption {
	return []wizardOption{
		{Label: "Yes, apply git defaults"},
		{Label: "No, leave git untouched"},
	}
}

func confirmOptions(mode Mode) []wizardOption {
	primary := "Yes, write profile and realize"
	if mode == ModeStandalone {
		primary = "Yes, write profile"
	}
	return []wizardOption{
		{Label: primary},
		{Label: "No, cancel"},
	}
}

func conflictOptions() []wizardOption {
	return []wizardOption{
		{Label: "Snapshot and continue"},
		{Label: "Abort"},
	}
}
