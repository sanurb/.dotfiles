package ui

import (
	"fmt"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/components/screen"
	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// layoutBuilders dispatches each stepID to the method that renders it.
// Method values bind at table-definition time; new steps are added by
// inserting one row, not by editing a switch.
var layoutBuilders = map[stepID]func(Model) screen.Layout{
	stepWelcome:       Model.welcomeLayout,
	stepShell:         Model.pillarLayout,
	stepTerminal:      Model.pillarLayout,
	stepFont:          Model.pillarLayout,
	stepMultiplexer:   Model.pillarLayout,
	stepEditor:        Model.pillarLayout,
	stepConfirm:       Model.confirmLayout,
	stepConflict:      Model.conflictLayout,
	stepScanning:      Model.scanningLayout,
	stepSnapshotting:  Model.snapshottingLayout,
	stepRealizePrompt: Model.realizePromptLayout,
	stepDone:          Model.doneLayout,
	stepFailed:        Model.failedLayout,
	stepAborted:       Model.abortedLayout,
}

func (m Model) layout() screen.Layout {
	if f, ok := layoutBuilders[m.step]; ok {
		return f(m)
	}
	return screen.Layout{}
}

// stepOptions is the option set every selection-style step renders.
// Three-row sets pull from the public Option lists (pillars); the rest
// are the mode-aware Yes/No-shaped wizardOptions defined below. Steps
// not in the map render no rows (transient/outcome surfaces).
var stepOptions = map[stepID]func(Model) []wizardOption{
	stepWelcome:       func(m Model) []wizardOption { return welcomeOptions(m.mode) },
	stepShell:         func(Model) []wizardOption { return asWizardOptions(ShellOptions) },
	stepTerminal:      func(Model) []wizardOption { return asWizardOptions(TerminalOptions) },
	stepFont:          func(Model) []wizardOption { return fontOptions() },
	stepMultiplexer:   func(Model) []wizardOption { return asWizardOptions(MultiplexerOptions) },
	stepEditor:        func(Model) []wizardOption { return editorOptions() },
	stepConfirm:       func(m Model) []wizardOption { return confirmOptions(m.mode) },
	stepConflict:      func(Model) []wizardOption { return conflictOptions() },
	stepRealizePrompt: func(Model) []wizardOption { return realizePromptOptions() },
}

func (m Model) currentRows() []screen.Row {
	if f, ok := stepOptions[m.step]; ok {
		return optionRows(f(m), m.cursor)
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

// pillarTexts is the title + description pair for each pillar step.
// Lookup-only; pillarCopy returns the zero pair for non-pillar steps.
var pillarTexts = map[stepID]struct{ title, desc string }{
	stepShell:       {"Shell", "One shell defines the persona. Atuin, zoxide, and starship are baked in regardless."},
	stepTerminal:    {"Terminal Emulator", "Pick the terminal app to configure."},
	stepFont:        {"Font", "Iosevka Nerd Font (required for icons)."},
	stepMultiplexer: {"Multiplexer", "Pick one, or 'None' to skip."},
	stepEditor:      {"Neovim", "Includes LSP, TreeSitter, and the dots Neovim config."},
}

func pillarCopy(s stepID) (title, desc string) {
	t := pillarTexts[s]
	return t.title, t.desc
}

// -- confirm --------------------------------------------------------

func (m Model) confirmLayout() screen.Layout {
	idx := stepperIndex(stepConfirm)
	desc := fmt.Sprintf("persona: %s   capabilities: %s",
		m.personaLine(), confirmCapabilities(m.state.Capabilities))

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

// -- transient (scan / snapshot) -----------------------------------

func (m Model) transientLayout(message string) screen.Layout {
	return screen.Layout{
		State:     screen.StateLoading,
		StatusMsg: m.spinner.View() + " " + message,
		Keymap:    screen.Keymap{Quit: "Ctrl-C"},
		Width:     m.width,
	}
}

func (m Model) scanningLayout() screen.Layout {
	return m.transientLayout("Scanning $HOME for brownfield collisions…")
}

func (m Model) snapshottingLayout() screen.Layout {
	return m.transientLayout("Snapshotting colliding paths into ~/.dots_backups…")
}

// -- realize prompt ------------------------------------------------

// realizePromptLayout is the forward-only consent gate between scan/
// snapshot and realization. Esc is intentionally absent — the persona
// is already persisted, so backing out would suggest editability that
// no longer exists.
func (m Model) realizePromptLayout() screen.Layout {
	km := screen.DefaultKeymap(false)
	km.SelectVerb = "confirm"
	km.Quit = "Ctrl-C"

	return screen.Layout{
		Title:       "Realize now?",
		Description: fmt.Sprintf("Will run %s in a subprocess. Output streams to this terminal.", ApplyCommand),
		Content:     screen.RenderRows(m.currentRows()),
		Keymap:      km,
		Width:       m.width,
	}
}

// -- outcome surfaces ----------------------------------------------

func (m Model) doneLayout() screen.Layout {
	heading, title := "✓", "Profile saved"
	if m.realizeRequested {
		title = "Profile saved — realizing next"
	}
	return screen.Layout{
		Heading:  heading,
		Title:    title,
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
	if mode == ModeSync {
		verb = "Re-realize"
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

func fontOptions() []wizardOption {
	return []wizardOption{
		{Label: "Yes, install Iosevka Nerd Font"},
		{Label: "No, I already have it"},
	}
}

func confirmOptions(_ Mode) []wizardOption {
	// Confirm writes the profile only; realization is gated by a
	// separate prompt (stepRealizePrompt) so the persona-commit and
	// system-mutation decisions stay distinct. The mode parameter is
	// preserved on the signature so the dispatch table in stepOptions
	// stays uniform with the other Mode-aware option builders.
	return []wizardOption{
		{Label: "Yes, write profile and continue"},
		{Label: "No, cancel"},
	}
}

func conflictOptions() []wizardOption {
	return []wizardOption{
		{Label: "Snapshot and continue"},
		{Label: "Abort"},
	}
}

func realizePromptOptions() []wizardOption {
	return []wizardOption{
		{Label: "Yes, run dots deploy now"},
		{Label: "No, exit — I'll run dots deploy myself"},
	}
}

// confirmCapabilities renders the capability list for stepConfirm's
// description. Uses "neovim" rather than the renderDoneSummary "editor"
// label because the Confirm screen is showing the user the *choice*
// they made on stepEditor, where the label was "Neovim".
func confirmCapabilities(c state.Capabilities) string {
	var picks []string
	if c.Font {
		picks = append(picks, "font")
	}
	if c.Editor {
		picks = append(picks, "neovim")
	}
	if len(picks) == 0 {
		return "(none)"
	}
	return strings.Join(picks, ", ")
}
