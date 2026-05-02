// Package stepview renders the shared frame for a wizard step in the
// dots TUI installer: breadcrumb + step counter + title/subtitle +
// content slot + keybind footer. The four legibility cues (flow
// position, cursor, committed selection, auto-detected default) each
// get a distinct visual treatment, plus explicit Loading/Empty/Error
// states, scroll cues for long lists, a help overlay, and a typed
// Keymap that stays stable across every step. See DESIGN.md.
package stepview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	glyphCrumbDone     = "●"
	glyphCrumbCurrent  = "◉"
	glyphCrumbUpcoming = "○"
	glyphCursor        = "▸"
	glyphChecked       = "[x]"
	glyphUnchecked     = "[ ]"
	glyphDefault       = "★"
	glyphMoreAbove     = "▲"
	glyphMoreBelow     = "▼"
	glyphLoading       = "◐"
	glyphError         = "✗"
)

// Mirrors the tokyonight tokens used by internal/ui/styles.go. Same hex
// values, no new color tokens.
const (
	colFg      lipgloss.Color = "#c0caf5"
	colMuted   lipgloss.Color = "#565f89"
	colAccent  lipgloss.Color = "#7aa2f7"
	colMagenta lipgloss.Color = "#bb9af7"
	colSuccess lipgloss.Color = "#9ece6a"
	colWarn    lipgloss.Color = "#e0af68"
	colError   lipgloss.Color = "#f7768e"
	colCyan    lipgloss.Color = "#7dcfff"
)

var (
	styCrumbDone     = lipgloss.NewStyle().Foreground(colSuccess)
	styCrumbCurrent  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styCrumbUpcoming = lipgloss.NewStyle().Foreground(colMuted)

	styCounter  = lipgloss.NewStyle().Foreground(colMuted)
	styTitle    = lipgloss.NewStyle().Foreground(colMagenta).Bold(true)
	stySubtitle = lipgloss.NewStyle().Foreground(colMuted)

	styCursor       = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styCheckBoxOn   = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styCheckBoxOff  = lipgloss.NewStyle().Foreground(colMuted)
	styRowSelected  = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styRow          = lipgloss.NewStyle().Foreground(colFg)
	styDefaultBadge = lipgloss.NewStyle().Foreground(colWarn)

	styScroll  = lipgloss.NewStyle().Foreground(colMuted).Italic(true)
	styLoading = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styEmpty   = lipgloss.NewStyle().Foreground(colMuted).Italic(true)
	styError   = lipgloss.NewStyle().Foreground(colError).Bold(true)

	styKey  = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	styHelp = lipgloss.NewStyle().Foreground(colMuted)
)

// minWidth is the floor used when the model's Width is unset. The
// design target is 80×24; below that joinSpread stacks the header.
const minWidth = 80

// State controls what occupies the content slot.
type State int

const (
	StateReady   State = iota // render Model.Content
	StateLoading              // render "◐ <msg>"
	StateEmpty                // render muted "<msg>"
	StateError                // render "✗ <msg>" in red
)

// Row is one line of selectable content. Cursor, Selected, and Default
// are independent flags — any combination is valid.
type Row struct {
	Label    string
	Cursor   bool
	Selected bool
	Default  bool
}

// Keymap is the canonical binding contract. Every step renders the
// same slots in the same order so the user never re-learns the footer.
// An empty string in any slot omits that footer entry.
//
//   Move      navigation keys, e.g. "↑/↓"
//   Select    commit, e.g. "enter"
//   Back      return to previous step, e.g. "esc"; "" on step 1
//   Help      toggle help overlay, e.g. "?"
//   Quit      exit the wizard, e.g. "q"
//   SelectVerb overrides the "select" label (e.g. "confirm" on step 6)
type Keymap struct {
	Move       string
	Select     string
	Back       string
	Help       string
	Quit       string
	SelectVerb string
}

// DefaultKeymap returns the conventional bindings for any step. Pass
// includeBack=false on step 1, where there is nowhere to go back.
func DefaultKeymap(includeBack bool) Keymap {
	km := Keymap{
		Move:   "↑/↓",
		Select: "enter",
		Help:   "?",
		Quit:   "q",
	}
	if includeBack {
		km.Back = "esc"
	}
	return km
}

// Viewport renders a list with scroll indicators when the row count
// exceeds the visible height. Cursor is the 0-indexed focused row.
// A Height of 0 disables truncation (renders all rows).
type Viewport struct {
	Rows   []Row
	Cursor int
	Height int
}

// Render returns the windowed list and 1-indexed (pos, total). When
// the cursor position carries no meaning (no rows) pos is 0.
func (v Viewport) Render() (body string, pos, total int) {
	total = len(v.Rows)
	if total == 0 {
		return "", 0, 0
	}
	pos = v.Cursor + 1

	if v.Height <= 0 || total <= v.Height {
		return RenderRows(v.Rows), pos, total
	}

	half := v.Height / 2
	start := v.Cursor - half
	if start < 0 {
		start = 0
	}
	end := start + v.Height
	if end > total {
		end = total
		start = end - v.Height
	}

	lines := make([]string, 0, v.Height+2)
	if start > 0 {
		lines = append(lines, styScroll.Render(fmt.Sprintf("%s %d more above", glyphMoreAbove, start)))
	} else {
		lines = append(lines, "")
	}
	for i := start; i < end; i++ {
		lines = append(lines, renderRow(v.Rows[i]))
	}
	if end < total {
		lines = append(lines, styScroll.Render(fmt.Sprintf("%s %d more below", glyphMoreBelow, total-end)))
	} else {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n"), pos, total
}

// Model is the frame state. Steps is the total count, Current is
// 0-indexed. ListPos / ListTotal show the cursor's position inside a
// long list (rendered next to the title); leave both 0 to hide.
type Model struct {
	Steps     int
	Current   int
	Title     string
	Subtitle  string
	Content   string
	State     State
	StatusMsg string
	Help      bool
	Keymap    Keymap
	ListPos   int
	ListTotal int
	Width     int
}

// View renders the full step frame.
func (m Model) View() string {
	width := m.Width
	if width <= 0 {
		width = minWidth
	}

	counter := styCounter.Render(fmt.Sprintf("Step %d of %d", m.Current+1, m.Steps))
	header := joinSpread(width, counter, renderBreadcrumb(m.Steps, m.Current))

	title := styTitle.Render(m.Title)
	if m.ListTotal > 0 {
		title = title + styCounter.Render(fmt.Sprintf("  (%d/%d)", m.ListPos, m.ListTotal))
	}

	out := []string{header, "", title}
	if m.Subtitle != "" {
		out = append(out, stySubtitle.Render(m.Subtitle))
	}

	body := m.body()
	out = append(out, "", body, "", renderFooter(m.Keymap))
	return strings.Join(out, "\n")
}

func (m Model) body() string {
	if m.Help {
		return renderHelp(m.Keymap)
	}
	switch m.State {
	case StateLoading:
		msg := m.StatusMsg
		if msg == "" {
			msg = "Loading…"
		}
		return styLoading.Render(glyphLoading + " " + msg)
	case StateEmpty:
		msg := m.StatusMsg
		if msg == "" {
			msg = "No items to show."
		}
		return styEmpty.Render(msg)
	case StateError:
		msg := m.StatusMsg
		if msg == "" {
			msg = "Something went wrong."
		}
		return styError.Render(glyphError + " " + msg)
	}
	return m.Content
}

func renderBreadcrumb(total, current int) string {
	parts := make([]string, 0, total)
	for i := 0; i < total; i++ {
		switch {
		case i < current:
			parts = append(parts, styCrumbDone.Render(glyphCrumbDone))
		case i == current:
			parts = append(parts, styCrumbCurrent.Render(glyphCrumbCurrent))
		default:
			parts = append(parts, styCrumbUpcoming.Render(glyphCrumbUpcoming))
		}
	}
	return strings.Join(parts, " ")
}

// joinSpread places left and right on one line padded to width. If the
// terminal cannot fit both, the breadcrumb wraps to its own line so
// neither piece clips.
func joinSpread(width int, left, right string) string {
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		return left + "\n" + right
	}
	return left + strings.Repeat(" ", pad) + right
}

// renderFooter emits the canonical key order: Move, Select, Back,
// Help, Quit. Empty slots are skipped. Quit is always present in
// DefaultKeymap so every step has an exit.
func renderFooter(km Keymap) string {
	verb := km.SelectVerb
	if verb == "" {
		verb = "select"
	}
	type slot struct{ key, help string }
	slots := []slot{
		{km.Move, "move"},
		{km.Select, verb},
		{km.Back, "back"},
		{km.Help, "help"},
		{km.Quit, "quit"},
	}
	parts := make([]string, 0, len(slots))
	for _, s := range slots {
		if s.key == "" {
			continue
		}
		parts = append(parts, styKey.Render(s.key)+" "+styHelp.Render(s.help))
	}
	return strings.Join(parts, "   ")
}

// renderHelp builds the overlay. Same key order as the footer. Keys
// are right-padded so the help column aligns.
func renderHelp(km Keymap) string {
	verb := km.SelectVerb
	if verb == "" {
		verb = "Commit selection"
	} else {
		verb = capitalize(verb)
	}
	type entry struct{ keys, help string }
	es := []entry{}
	if km.Move != "" {
		es = append(es, entry{km.Move + "   k/j", "Move cursor"})
	}
	if km.Select != "" {
		es = append(es, entry{km.Select, verb})
	}
	if km.Back != "" {
		es = append(es, entry{km.Back, "Return to previous step (selections preserved)"})
	}
	if km.Help != "" {
		es = append(es, entry{km.Help, "Toggle this help"})
	}
	if km.Quit != "" {
		es = append(es, entry{km.Quit, "Quit immediately"})
	}

	maxK := 0
	for _, e := range es {
		if w := lipgloss.Width(e.keys); w > maxK {
			maxK = w
		}
	}
	out := []string{styCounter.Render("Keys")}
	for _, e := range es {
		pad := strings.Repeat(" ", maxK-lipgloss.Width(e.keys))
		out = append(out, "  "+styKey.Render(e.keys)+pad+"   "+styHelp.Render(e.help))
	}
	return strings.Join(out, "\n")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// RenderRows renders a list of selectable rows using the cursor /
// selection / default contract. Callers feed the result into
// Model.Content. For long lists, prefer Viewport.
func RenderRows(rows []Row) string {
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = renderRow(r)
	}
	return strings.Join(lines, "\n")
}

func renderRow(r Row) string {
	cursor := " "
	if r.Cursor {
		cursor = styCursor.Render(glyphCursor)
	}

	box := styCheckBoxOff.Render(glyphUnchecked)
	label := styRow.Render(r.Label)
	if r.Selected {
		box = styCheckBoxOn.Render(glyphChecked)
		label = styRowSelected.Render(r.Label)
	}

	line := cursor + " " + box + " " + label
	if r.Default {
		line += " " + styDefaultBadge.Render(glyphDefault)
	}
	return line
}
