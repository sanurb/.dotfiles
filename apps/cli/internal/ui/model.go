// Package ui hosts the dots install/sync/standalone wizard. The
// dispatcher in this file owns wizard state and routes key events to
// the active step; the visual frame for every step is built from the
// reusable primitives in internal/tui/components/screen and the tokens
// in internal/tui/theme — no inline styling, no per-step layout.
//
// Realization (the `nh home switch` invocation) is NOT performed by
// this wizard. After the user consents on stepRealizePrompt the wizard
// exits with Result.RealizeRequested = true; main.go re-invokes the
// dots binary as `dots deploy` so the realization driver renders
// against the real terminal instead of inside an alt-screen TUI. See
// ADR-0009.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/components/screen"
	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

type stepID int

const (
	stepWelcome stepID = iota
	stepShell
	stepTerminal
	stepMultiplexer
	stepEditor
	stepGit
	stepConfirm
	stepScanning
	stepConflict
	stepSnapshotting
	stepRealizePrompt
	stepDone
	stepFailed
	stepAborted
)

// stepperLabels are the six labels shown in the top stepper during the
// install flow (post-welcome, pre-scan).
var stepperLabels = []string{"Shell", "Terminal", "Multiplexer", "Editor", "Git", "Confirm"}

// Model is the wizard's bubbletea model.
type Model struct {
	mode Mode
	deps Deps
	step stepID

	spinner spinner.Model

	state           state.State
	formShell       string
	formTerminal    string
	formMultiplexer string
	formEditor      bool
	formGit         bool

	cursor int

	collisions       []Collision
	snapshotResult   SnapshotResult
	realizeRequested bool

	err error

	width  int
	height int
}

// New returns a fully wired model ready for tea.NewProgram.
func New(mode Mode, deps Deps) *Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = theme.Spinner

	initial := deps.Initial
	if initial.SchemaVersion == 0 {
		initial = state.Default()
	}

	m := &Model{
		mode:            mode,
		deps:            deps,
		spinner:         sp,
		state:           initial,
		formShell:       initial.Pillars.Shell,
		formTerminal:    initial.Pillars.Terminal,
		formMultiplexer: initial.Pillars.Multiplexer,
		formEditor:      initial.Capabilities.Editor,
		formGit:         initial.Capabilities.Git,
	}

	switch mode {
	case ModeSync:
		m.step = stepScanning
	default:
		m.step = stepWelcome
	}
	m.cursor = m.initialCursor(m.step)
	return m
}

func (m Model) Init() tea.Cmd {
	switch m.step {
	case stepScanning:
		return tea.Batch(m.spinner.Tick, m.scanCmd())
	default:
		return m.spinner.Tick
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.step = stepAborted
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanCompleteMsg:
		m.collisions = msg.collisions
		if len(m.collisions) == 0 {
			return m.afterSnapshotOrSkip()
		}
		return m.transition(stepConflict), nil

	case scanFailedMsg:
		m.err = msg.err
		return m.fail()

	case snapshotCompleteMsg:
		m.snapshotResult = SnapshotResult(msg)
		return m.afterSnapshotOrSkip()

	case snapshotFailedMsg:
		m.err = msg.err
		return m.fail()
	}

	return m, nil
}

func (m Model) View() string {
	return screen.Render(m.layout())
}

// -- key handling ---------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome, stepShell, stepTerminal, stepMultiplexer,
		stepEditor, stepGit, stepConfirm, stepConflict, stepRealizePrompt:
		return m.handleSelectKey(msg)

	case stepDone, stepFailed, stepAborted:
		switch msg.String() {
		case "enter", " ", "esc", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) handleSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.currentRows()
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(rows)-1 {
			m.cursor++
		}
	case "enter":
		return m.commit()
	case "esc":
		return m.back()
	case "q":
		m.step = stepAborted
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) commit() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		if m.cursor == 1 {
			m.step = stepAborted
			return m, tea.Quit
		}
		switch m.mode {
		case ModeInstall, ModeStandalone:
			return m.transition(stepShell), nil
		case ModeSync:
			return m.startScan()
		}

	case stepShell:
		m.formShell = ShellOptions[m.cursor].Value
		return m.transition(stepTerminal), nil

	case stepTerminal:
		m.formTerminal = TerminalOptions[m.cursor].Value
		return m.transition(stepMultiplexer), nil

	case stepMultiplexer:
		m.formMultiplexer = MultiplexerOptions[m.cursor].Value
		return m.transition(stepEditor), nil

	case stepEditor:
		m.formEditor = m.cursor == 0
		return m.transition(stepGit), nil

	case stepGit:
		m.formGit = m.cursor == 0
		return m.transition(stepConfirm), nil

	case stepConfirm:
		if m.cursor != 0 {
			m.step = stepAborted
			return m, tea.Quit
		}
		return m.persistAndAdvance()

	case stepConflict:
		if m.cursor != 0 {
			m.step = stepAborted
			return m, tea.Quit
		}
		return m.startSnapshot()

	case stepRealizePrompt:
		// Row 0: Yes — realize via subprocess after the wizard exits.
		// Row 1: No — exit cleanly with stepDone; main prints the
		// "Profile saved. Run `dots deploy` when ready." reminder.
		m.realizeRequested = m.cursor == 0
		return m.transition(stepDone), tea.Quit
	}
	return m, nil
}

// back returns to the previous step. Pillar selections persist in
// formXxx fields independently of cursor; transition() reseeds cursor
// from the captured value so the user sees their prior choice highlighted.
//
// stepRealizePrompt is intentionally not reachable via Esc — once the
// state file is persisted, the prompt is a forward-only consent gate.
// Backing out would suggest the persona could be re-edited, which it
// can't from here.
func (m Model) back() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepTerminal:
		return m.transition(stepShell), nil
	case stepMultiplexer:
		return m.transition(stepTerminal), nil
	case stepEditor:
		return m.transition(stepMultiplexer), nil
	case stepGit:
		return m.transition(stepEditor), nil
	case stepConfirm:
		return m.transition(stepGit), nil
	}
	return m, nil
}

// -- transitions ----------------------------------------------------

func (m Model) transition(s stepID) Model {
	m.step = s
	m.cursor = m.initialCursor(s)
	return m
}

func (m Model) initialCursor(s stepID) int {
	switch s {
	case stepShell:
		return indexOf(ShellOptions, m.formShell)
	case stepTerminal:
		return indexOf(TerminalOptions, m.formTerminal)
	case stepMultiplexer:
		return indexOf(MultiplexerOptions, m.formMultiplexer)
	case stepEditor:
		if m.formEditor {
			return 0
		}
		return 1
	case stepGit:
		if m.formGit {
			return 0
		}
		return 1
	case stepRealizePrompt:
		return 0 // default cursor on Yes — affirmative is the install-flow happy path
	}
	return 0
}

// persistAndAdvance copies the captured persona into m.state, writes
// it via StatePersister, and starts the scan. ModeStandalone short-
// circuits to Done after writing — there is no workspace to scan or
// realize against.
func (m Model) persistAndAdvance() (tea.Model, tea.Cmd) {
	m.state.Pillars.Shell = m.formShell
	m.state.Pillars.Terminal = m.formTerminal
	m.state.Pillars.Multiplexer = m.formMultiplexer
	m.state.Capabilities.Editor = m.formEditor
	m.state.Capabilities.Git = m.formGit

	if m.deps.StatePersister != nil {
		if err := m.deps.StatePersister.SaveState(m.state); err != nil {
			m.err = fmt.Errorf("persist state: %w", err)
			return m.fail()
		}
	}

	if m.mode == ModeStandalone {
		return m.transition(stepDone), tea.Quit
	}
	return m.startScan()
}

// afterSnapshotOrSkip is the post-scan / post-snapshot fork. Sync mode
// auto-realizes (the user invoked `dots sync` precisely to re-realize,
// re-prompting is friction); install mode asks one more time before
// mutating the system.
func (m Model) afterSnapshotOrSkip() (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeSync:
		m.realizeRequested = true
		return m.transition(stepDone), tea.Quit
	default:
		return m.transition(stepRealizePrompt), nil
	}
}

func (m Model) startScan() (tea.Model, tea.Cmd) {
	next := m.transition(stepScanning)
	return next, tea.Batch(next.spinner.Tick, next.scanCmd())
}

func (m Model) startSnapshot() (tea.Model, tea.Cmd) {
	next := m.transition(stepSnapshotting)
	return next, tea.Batch(next.spinner.Tick, next.snapshotCmd())
}

func (m Model) fail() (tea.Model, tea.Cmd) {
	m.step = stepFailed
	return m, tea.Quit
}

// -- commands (execution side) --------------------------------------

func (m Model) scanCmd() tea.Cmd {
	deps := m.deps
	return func() tea.Msg {
		cs, err := deps.Snapshotter.Scan()
		if err != nil {
			return scanFailedMsg{err: err}
		}
		return scanCompleteMsg{collisions: cs}
	}
}

func (m Model) snapshotCmd() tea.Cmd {
	deps := m.deps
	cs := m.collisions
	return func() tea.Msg {
		res, err := deps.Snapshotter.Snapshot(cs)
		if err != nil {
			return snapshotFailedMsg{err: err}
		}
		return snapshotCompleteMsg(res)
	}
}

// -- helpers --------------------------------------------------------

func indexOf(opts []Option, value string) int {
	for i, o := range opts {
		if o.Value == value {
			return i
		}
	}
	return 0
}

// renderDoneSummary builds the post-wizard "what just happened" block
// for stepDone (the only screen wrapped in OutcomePanel). The deploy
// status is reported by main.go after the subprocess returns — this
// function only summarizes what the wizard itself decided.
func (m Model) renderDoneSummary() string {
	var b strings.Builder
	if m.snapshotResult.Count > 0 {
		fmt.Fprintf(&b, "  %s snapshot %s %s\n",
			theme.BadgeOK,
			theme.Muted.Render("→"),
			theme.Body.Render(fmt.Sprintf("%d path(s) → %s", m.snapshotResult.Count, m.snapshotResult.Path)))
	}

	persona := fmt.Sprintf("%s · %s · %s",
		m.state.Pillars.Shell, m.state.Pillars.Terminal, m.state.Pillars.Multiplexer)
	extras := []string{}
	if m.state.Capabilities.Editor {
		extras = append(extras, "editor")
	}
	if m.state.Capabilities.Git {
		extras = append(extras, "git")
	}
	extrasLine := "(none)"
	if len(extras) > 0 {
		extrasLine = strings.Join(extras, ", ")
	}

	fmt.Fprintf(&b, "  %s persona %s %s\n",
		theme.BadgeOK, theme.Muted.Render("→"), theme.Body.Render(persona))
	fmt.Fprintf(&b, "  %s extras  %s %s\n",
		theme.BadgeOK, theme.Muted.Render("→"), theme.Body.Render(extrasLine))

	switch {
	case m.mode == ModeStandalone:
		fmt.Fprintf(&b, "  %s deploy  %s %s\n",
			theme.BadgeWarn, theme.Muted.Render("→"),
			theme.Muted.Render("not run (no workspace)"))
	case m.realizeRequested:
		fmt.Fprintf(&b, "  %s deploy  %s %s\n",
			theme.BadgeOK, theme.Muted.Render("→"),
			theme.Muted.Render("running "+MoonRunDeploy+"…"))
	default:
		fmt.Fprintf(&b, "  %s deploy  %s %s\n",
			theme.BadgeWarn, theme.Muted.Render("→"),
			theme.Muted.Render("deferred — run `dots deploy` when ready"))
	}

	return strings.TrimRight(b.String(), "\n")
}
