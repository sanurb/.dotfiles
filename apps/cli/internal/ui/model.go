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

// Model is the wizard's bubbletea model. m.state is the single source
// of truth for the persona — pillar selections write through it
// directly; the on-disk file is only touched at persistAndAdvance.
// Aborts (Esc, Cancel, Ctrl-C) discard the in-memory mutations because
// the process exits.
type Model struct {
	mode Mode
	deps Deps
	step stepID

	spinner spinner.Model

	state  state.State
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
		mode:    mode,
		deps:    deps,
		spinner: sp,
		state:   initial,
	}

	if mode == ModeSync {
		m.step = stepScanning
	} else {
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

// selectionStep describes a step that captures one piece of persona
// state. apply mutates m.state from the cursor; next is the step the
// commit advances to. One row per pillar/capability — adding a step
// is a one-line table edit.
type selectionStep struct {
	apply func(s *state.State, cursor int)
	next  stepID
}

var selectionSteps = map[stepID]selectionStep{
	stepShell: {
		apply: func(s *state.State, c int) { s.Pillars.Shell = ShellOptions[c].Value },
		next:  stepTerminal,
	},
	stepTerminal: {
		apply: func(s *state.State, c int) { s.Pillars.Terminal = TerminalOptions[c].Value },
		next:  stepMultiplexer,
	},
	stepMultiplexer: {
		apply: func(s *state.State, c int) { s.Pillars.Multiplexer = MultiplexerOptions[c].Value },
		next:  stepEditor,
	},
	stepEditor: {
		apply: func(s *state.State, c int) { s.Capabilities.Editor = c == 0 },
		next:  stepGit,
	},
	stepGit: {
		apply: func(s *state.State, c int) { s.Capabilities.Git = c == 0 },
		next:  stepConfirm,
	},
}

func (m Model) commit() (tea.Model, tea.Cmd) {
	if def, ok := selectionSteps[m.step]; ok {
		def.apply(&m.state, m.cursor)
		return m.transition(def.next), nil
	}

	switch m.step {
	case stepWelcome:
		if m.cursor == 1 {
			m.step = stepAborted
			return m, tea.Quit
		}
		if m.mode == ModeSync {
			return m.startScan()
		}
		return m.transition(stepShell), nil

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
		m.realizeRequested = m.cursor == 0
		return m.transition(stepDone), tea.Quit
	}
	return m, nil
}

// previousStep is the Esc target for each step that supports it. Steps
// absent from the map have no back action. stepRealizePrompt is
// intentionally absent: once the state file is persisted, the prompt
// is a forward-only consent gate.
var previousStep = map[stepID]stepID{
	stepTerminal:    stepShell,
	stepMultiplexer: stepTerminal,
	stepEditor:      stepMultiplexer,
	stepGit:         stepEditor,
	stepConfirm:     stepGit,
}

func (m Model) back() (tea.Model, tea.Cmd) {
	if prev, ok := previousStep[m.step]; ok {
		return m.transition(prev), nil
	}
	return m, nil
}

// -- transitions ----------------------------------------------------

func (m Model) transition(s stepID) Model {
	m.step = s
	m.cursor = m.initialCursor(s)
	return m
}

// cursorSeeders restores the cursor to the row that matches the
// current persona on entry to a selection step (or on Esc-back). Steps
// absent from the map default to row 0, which is also the affirmative
// row for prompt-style steps (welcome, confirm, conflict,
// stepRealizePrompt).
var cursorSeeders = map[stepID]func(Model) int{
	stepShell:       func(m Model) int { return indexOf(ShellOptions, m.state.Pillars.Shell) },
	stepTerminal:    func(m Model) int { return indexOf(TerminalOptions, m.state.Pillars.Terminal) },
	stepMultiplexer: func(m Model) int { return indexOf(MultiplexerOptions, m.state.Pillars.Multiplexer) },
	stepEditor:      func(m Model) int { return boolToCursor(m.state.Capabilities.Editor) },
	stepGit:         func(m Model) int { return boolToCursor(m.state.Capabilities.Git) },
}

func (m Model) initialCursor(s stepID) int {
	if f, ok := cursorSeeders[s]; ok {
		return f(m)
	}
	return 0
}

// boolToCursor maps the affirmative/negative value of a Yes/No
// selection back to its row index. Yes = row 0; No = row 1.
func boolToCursor(yes bool) int {
	if yes {
		return 0
	}
	return 1
}

// persistAndAdvance writes the in-memory persona to disk and starts
// the scan. ModeStandalone short-circuits to Done — there is no
// workspace to scan or realize against.
func (m Model) persistAndAdvance() (tea.Model, tea.Cmd) {
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
