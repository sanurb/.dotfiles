package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
)

type step int

const (
	stepWelcome step = iota
	stepCaps
	stepScanning
	stepConflict
	stepSnapshotting
	stepRealizing
	stepDone
	stepFailed
	stepAborted
)

const (
	realizeEstimateSec = 25 // estimated deploy duration; controls progress ramp
	progressTickEvery  = 250 * time.Millisecond
)

// Model is the wizard's bubbletea model. Value-receiver Update returns
// a new copy on each transition (bubbletea best practice). The huh form
// is held as a pointer because that is huh's documented API surface.
type Model struct {
	mode Mode
	deps Deps
	step step

	form     *huh.Form
	spinner  spinner.Model
	progress progress.Model

	// Captured user input. The wizard mutates `state` in place from the
	// pillar form's bound fields and persists it via StatePersister
	// before kicking off the scan. After persistence, `state` is the
	// canonical record of what the deploy will realize.
	state           state.State
	formShell       string
	formTerminal    string
	formMultiplexer string
	formEditor      bool
	formGit         bool
	wantConfirm     bool // welcome / conflict confirmation
	doSnapshot      bool

	// Captured execution output.
	collisions     []Collision
	snapshotResult SnapshotResult
	realizeResult  RealizationResult
	err            error

	// Timing.
	startedAt time.Time
	width     int
	height    int
}

// New returns a fully wired model ready for tea.NewProgram.
func New(mode Mode, deps Deps) *Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = stySpinner

	pr := progress.New(
		progress.WithGradient(string(colAccent), string(colSuccess)),
		progress.WithoutPercentage(),
	)
	pr.Width = 50

	initial := deps.Initial
	if initial.SchemaVersion == 0 {
		// Adapter forgot to seed; fail safe to defaults rather than
		// writing zeroed pillars to disk.
		initial = state.Default()
	}

	m := &Model{
		mode:            mode,
		deps:            deps,
		spinner:         sp,
		progress:        pr,
		state:           initial,
		formShell:       initial.Pillars.Shell,
		formTerminal:    initial.Pillars.Terminal,
		formMultiplexer: initial.Pillars.Multiplexer,
		formEditor:      initial.Capabilities.Editor,
		formGit:         initial.Capabilities.Git,
	}
	m.transition(stepWelcome)
	return m
}

// Init runs once when the program starts. Value receiver because all
// other tea.Model methods (Update, View) are value receivers — and the
// language requires uniform receiver types for the interface to be
// satisfied by Model values that bubbletea passes back through Update.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.form.Init(), m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = min(60, msg.Width-10)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.step = stepAborted
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progressTickMsg:
		return m.handleProgressTick()
	case scanCompleteMsg:
		m.collisions = msg.collisions
		if len(m.collisions) == 0 {
			return m.startRealize()
		}
		return m.transitionAndCmd(stepConflict)
	case scanFailedMsg:
		m.err = msg.err
		return m.fail()
	case snapshotCompleteMsg:
		m.snapshotResult = SnapshotResult(msg)
		return m.startRealize()
	case snapshotFailedMsg:
		m.err = msg.err
		return m.fail()
	case realizeCompleteMsg:
		m.realizeResult = RealizationResult(msg)
		_ = m.progress.SetPercent(1.0)
		return m.transitionAndCmd(stepDone)
	case realizeFailedMsg:
		m.err = msg.err
		return m.fail()
	case progress.FrameMsg:
		newP, cmd := m.progress.Update(msg)
		p, _ := newP.(progress.Model)
		m.progress = p
		return m, cmd
	}

	if m.form != nil && m.formStep() {
		f, cmd := m.form.Update(msg)
		if updated, ok := f.(*huh.Form); ok {
			m.form = updated
		}
		if m.form.State == huh.StateCompleted {
			return m.advanceFromForm()
		}
		if m.form.State == huh.StateAborted {
			m.step = stepAborted
			return m, tea.Quit
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	switch m.step {
	case stepWelcome, stepCaps, stepConflict:
		return styPanel.Render(m.form.View())
	case stepScanning:
		return styPanel.Render(fmt.Sprintf("%s %s",
			m.spinner.View(),
			styBody.Render("Scanning $HOME for brownfield collisions…")))
	case stepSnapshotting:
		return styPanel.Render(fmt.Sprintf("%s %s",
			m.spinner.View(),
			styBody.Render("Snapshotting colliding paths into ~/.dots_backups…")))
	case stepRealizing:
		return styPanel.Render(strings.Join([]string{
			fmt.Sprintf("%s %s", m.spinner.View(), styBody.Render("Realizing Home Manager state…")),
			"",
			m.progress.View(),
			"",
			styMuted.Render(MoonRunDeploy),
		}, "\n"))
	case stepDone:
		return styPanel.Render(m.renderDone())
	case stepFailed:
		return styPanel.Render(m.renderFailed())
	case stepAborted:
		return styPanel.Render(styWarn.Render("Aborted."))
	}
	return ""
}

// -- transitions -----------------------------------------------------

func (m *Model) transition(s step) {
	m.step = s
	switch s {
	case stepWelcome:
		m.form = m.welcomeForm()
	case stepCaps:
		m.form = m.capabilitiesForm()
	case stepConflict:
		m.form = m.conflictForm()
	}
}

func (m Model) transitionAndCmd(s step) (tea.Model, tea.Cmd) {
	m.transition(s)
	if m.formStep() {
		return m, m.form.Init()
	}
	return m, nil
}

func (m Model) formStep() bool {
	return m.step == stepWelcome || m.step == stepCaps || m.step == stepConflict
}

// advanceFromForm is called when a huh form completes (StateCompleted).
// It reads the bound values and transitions to the next step, kicking
// off any background work via tea.Cmd.
func (m Model) advanceFromForm() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		if !m.wantConfirm {
			m.step = stepAborted
			return m, tea.Quit
		}
		if m.mode == ModeInstall {
			return m.transitionAndCmd(stepCaps)
		}
		return m.startScan()
	case stepCaps:
		// Pull the bound form values into the state struct, validate,
		// snapshot the prior file, and atomically write the new one.
		// This must happen before the deploy so that home.nix's
		// builtins.fromTOML sees the new persona.
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
		return m.startScan()
	case stepConflict:
		if !m.doSnapshot {
			m.step = stepAborted
			return m, tea.Quit
		}
		m.transition(stepSnapshotting)
		return m, tea.Batch(m.spinner.Tick, m.snapshotCmd())
	}
	return m, nil
}

func (m Model) startScan() (tea.Model, tea.Cmd) {
	m.transition(stepScanning)
	return m, tea.Batch(m.spinner.Tick, m.scanCmd())
}

func (m Model) startRealize() (tea.Model, tea.Cmd) {
	m.transition(stepRealizing)
	m.startedAt = time.Now()
	return m, tea.Batch(
		m.spinner.Tick,
		m.realizeCmd(),
		m.progressTickCmd(),
	)
}

func (m Model) handleProgressTick() (tea.Model, tea.Cmd) {
	if m.step != stepRealizing {
		return m, nil
	}
	elapsed := time.Since(m.startedAt).Seconds()
	pct := elapsed / float64(realizeEstimateSec)
	if pct > 0.95 {
		pct = 0.95
	}
	cmd := m.progress.SetPercent(pct)
	return m, tea.Batch(cmd, m.progressTickCmd())
}

func (m Model) fail() (tea.Model, tea.Cmd) {
	m.step = stepFailed
	return m, tea.Quit
}

// -- forms -----------------------------------------------------------

func (m *Model) welcomeForm() *huh.Form {
	title := "dots install"
	body := strings.Join([]string{
		"Realize the workspace's Home Manager state into ~/.",
		"",
		"This wizard will:",
		"  • capture your shell / terminal / multiplexer persona",
		"  • detect colliding paths under $HOME",
		"  • offer to snapshot them into ~/.dots_backups/<ts>",
		"  • run " + MoonRunDeploy + " (gated by cli:check)",
	}, "\n")
	if m.mode == ModeSync {
		title = "dots sync"
		body = strings.Join([]string{
			"Re-realize the workspace's Home Manager state into ~/.",
			"",
			"Sync respects the existing .dots-state.toml — it never",
			"re-prompts for the persona. To change shell/terminal/mux,",
			"run `dots install` instead.",
		}, "\n")
	}
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(title).Description(body),
			huh.NewConfirm().
				Title("Continue?").
				Affirmative("Yes").
				Negative("Cancel").
				Value(&m.wantConfirm),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

// capabilitiesForm collects the persona — three radio pillars plus
// per-capability Yes/No confirms. Atuin/Zoxide/Starship are
// intentionally absent: they are mandatory infrastructure
// (foundation.nix), not user choices.
func (m *Model) capabilitiesForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Shell").
				Description("One shell defines the persona. Atuin, Zoxide, and Starship are baked in regardless.").
				Options(HuhOptions(ShellOptions, 22)...).
				Value(&m.formShell),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Terminal Emulator").
				Options(HuhOptions(TerminalOptions, 22)...).
				Value(&m.formTerminal),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Multiplexer").
				Description("Pick one, or 'None' to skip.").
				Options(HuhOptions(MultiplexerOptions, 22)...).
				Value(&m.formMultiplexer),
		),
		huh.NewGroup(EditorConfirm(&m.formEditor)),
		huh.NewGroup(GitConfirm(&m.formGit)),
	).WithTheme(huh.ThemeCharm())
}

// EditorConfirm and GitConfirm are exported so the standalone install
// path in apps/cli/main can reuse the exact same copy as the wizard.
func EditorConfirm(value *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title("Neovim").
		Description("Includes LSP, TreeSitter, and Sanurb Config").
		Affirmative("Yes, install Neovim with config.").
		Negative("No, skip Neovim").
		Value(value)
}

func GitConfirm(value *bool) *huh.Confirm {
	return huh.NewConfirm().
		Title("Git defaults").
		Description("Global git config (identity stays external)").
		Affirmative("Yes, apply git defaults").
		Negative("No, leave git untouched").
		Value(value)
}

func (m *Model) conflictForm() *huh.Form {
	lines := make([]string, len(m.collisions))
	for i, c := range m.collisions {
		lines[i] = fmt.Sprintf("  • [%s] %s", c.Kind, c.Path)
	}
	desc := strings.Join(append([]string{
		"The following paths exist as real files/dirs and will collide",
		"with Home Manager symlinks. They can be safely moved into",
		"~/.dots_backups/<ts>/ — nothing is deleted.",
		"",
	}, lines...), "\n")
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Brownfield conflicts").Description(desc),
			huh.NewConfirm().
				Title("Snapshot and continue?").
				Affirmative("Snapshot").
				Negative("Abort").
				Value(&m.doSnapshot),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
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

func (m Model) realizeCmd() tea.Cmd {
	deps := m.deps
	return func() tea.Msg {
		res, err := deps.Realizer.Realize()
		if err != nil {
			return realizeFailedMsg{err: err}
		}
		return realizeCompleteMsg(res)
	}
}

func (m Model) progressTickCmd() tea.Cmd {
	return tea.Tick(progressTickEvery, func(t time.Time) tea.Msg {
		return progressTickMsg(t)
	})
}

// -- terminal renders -----------------------------------------------

func (m Model) renderDone() string {
	lines := []string{
		styTitle.Render("✓ Realized"),
	}
	if m.snapshotResult.Count > 0 {
		lines = append(lines,
			fmt.Sprintf("  %s snapshot %s %s",
				styBadgeOK,
				styMuted.Render("→"),
				styBody.Render(fmt.Sprintf("%d path(s) → %s", m.snapshotResult.Count, m.snapshotResult.Path))))
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
	lines = append(lines,
		fmt.Sprintf("  %s persona %s %s",
			styBadgeOK,
			styMuted.Render("→"),
			styBody.Render(persona)),
		fmt.Sprintf("  %s extras  %s %s",
			styBadgeOK,
			styMuted.Render("→"),
			styBody.Render(extrasLine)),
		fmt.Sprintf("  %s deploy  %s %s",
			styBadgeOK,
			styMuted.Render("→"),
			styBody.Render(fmt.Sprintf("%s in %.1fs", MoonRunDeploy, float64(m.realizeResult.DurationMs)/1000.0))),
		"",
		styMuted.Render("Open a new shell to pick up changes."),
	)
	return strings.Join(lines, "\n")
}

func (m Model) renderFailed() string {
	hint := "Run `dots doctor` to inspect toolchain state."
	return strings.Join([]string{
		styError.Render("✗ Failed"),
		"",
		styBody.Render(m.err.Error()),
		"",
		styMuted.Render(hint),
	}, "\n")
}

// -- helpers --------------------------------------------------------

// HuhOptions renders Option labels for a huh select/multiselect.
// labelWidth left-pads the label so the " — description" column lines
// up across rows; pass 0 to skip padding (single-form, narrow layout).
// Exported so the standalone install path in apps/cli/main can reuse
// the same formatting as the wizard's capabilitiesForm.
func HuhOptions(opts []Option, labelWidth int) []huh.Option[string] {
	out := make([]huh.Option[string], len(opts))
	for i, o := range opts {
		label := fmt.Sprintf("%-*s — %s", labelWidth, o.Label, o.Description)
		out[i] = huh.NewOption(label, o.Value)
	}
	return out
}
