// Package screen is the layout primitive for every wizard step. A
// Layout describes the slots; Render returns the final string. The
// primitive itself is opinionated about ordering — stepper, heading,
// description, content, optional auxiliary, footer — so screens cannot
// reorder visual structure away from DESIGN.md by accident.
//
// Wizard screens are borderless; outcome surfaces (done, failed,
// doctor) wrap their string in theme.OutcomePanel themselves. The
// reasoning is in DESIGN.md §components.panel.
package screen

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// MinWidth is the floor used when Layout.Width is unset. Below this,
// the stepper may wrap onto its own line; below 80 columns generally
// is allowed-to-clip per DESIGN.md minimum_size.
const MinWidth = 80

// Stepper is the linear progress indicator at the top of a wizard
// screen. Steps is the ordered list of step labels; Current is the
// 0-indexed step the user is on. The primitive owns rendering — never
// draw a stepper outside this package.
type Stepper struct {
	Steps   []string
	Current int
}

// State controls what occupies the content slot when Layout.Content is
// empty. Default StateReady renders Layout.Content verbatim.
type State int

const (
	StateReady   State = iota // render Layout.Content
	StateLoading              // "<spinner-glyph> <msg>"
	StateEmpty                // muted "<msg>"
	StateError                // "✗ <msg>" in red
)

// Layout describes one wizard screen.
//
//	Stepper:     optional progress strip; nil to omit (welcome, outcome surfaces)
//	Heading:     the "Step N:" prefix or section label; muted
//	Title:       the focal title in accent.title; required
//	Description: muted single line beneath the title; "" to omit
//	Content:     pre-rendered options/prose for the State == StateReady path
//	State:       overrides Content with loading/empty/error semantics
//	StatusMsg:   message shown when State != StateReady
//	Auxiliary:   non-focusable affordance rows; nil/empty to omit the rule
//	Keymap:      footer; empty Keymap renders no footer (rare)
//	Width:       0 = MinWidth; viewport reflows accordingly
//	Bordered:    outcome surfaces only; wizard screens leave false
type Layout struct {
	Stepper     *Stepper
	Heading     string
	Title       string
	Description string
	Content     string
	State       State
	StatusMsg   string
	Auxiliary   []AuxRow
	Keymap      Keymap
	Width       int
	Bordered    bool
}

// AuxRow is one row of the optional auxiliary action panel below the
// option list. Icon is one cell (emoji or block glyph); Label is body
// text; Key is the mnemonic (e.g. "i") shown to the user. Non-focusable
// in cursor flow — the dispatcher binds Key to a screen-local action.
type AuxRow struct {
	Icon  string
	Label string
	Key   string
}

// Render returns the full screen string in the canonical slot order.
// Layout.Width is reserved for future width-responsive reflow inside
// the body; today it is only consulted by callers when sizing the
// list viewport before composing Content.
func Render(l Layout) string {
	var parts []string

	if l.Stepper != nil {
		parts = append(parts, renderStepper(*l.Stepper))
		parts = append(parts, "")
	}

	if l.Heading != "" || l.Title != "" {
		parts = append(parts, renderTitle(l.Heading, l.Title))
	}
	if l.Description != "" {
		parts = append(parts, theme.Muted.Render(l.Description))
	}

	parts = append(parts, "")
	parts = append(parts, renderBody(l))

	if len(l.Auxiliary) > 0 {
		parts = append(parts, "")
		parts = append(parts, renderAuxiliary(l.Auxiliary))
	}

	if !l.Keymap.Empty() {
		parts = append(parts, "")
		parts = append(parts, l.Keymap.Footer())
	}

	out := strings.Join(parts, "\n")
	if l.Bordered {
		return theme.OutcomePanel.Render(out)
	}
	return out
}

func renderTitle(heading, title string) string {
	if heading == "" {
		return theme.Title.Render(title)
	}
	if title == "" {
		return theme.Muted.Render(heading)
	}
	return theme.Muted.Render(heading+" ") + theme.Title.Render(title)
}

func renderBody(l Layout) string {
	switch l.State {
	case StateLoading:
		msg := l.StatusMsg
		if msg == "" {
			msg = "Loading…"
		}
		return theme.Accent.Render(theme.GlyphLoading + " " + msg)
	case StateEmpty:
		msg := l.StatusMsg
		if msg == "" {
			msg = "No items to show."
		}
		return theme.Muted.Render(msg)
	case StateError:
		msg := l.StatusMsg
		if msg == "" {
			msg = "Something went wrong."
		}
		return theme.Error.Render(theme.GlyphBadgeFail + " " + msg)
	}
	return l.Content
}

func renderStepper(s Stepper) string {
	parts := make([]string, 0, len(s.Steps))
	for i, label := range s.Steps {
		switch {
		case i < s.Current:
			parts = append(parts, theme.Success.Render(theme.GlyphStepDone+" "+label))
		case i == s.Current:
			parts = append(parts, theme.Accent.Render(theme.GlyphStepCurrent+" "+label))
		default:
			parts = append(parts, theme.Muted.Render(label))
		}
	}
	sep := theme.Muted.Render(" → ")
	return strings.Join(parts, sep)
}

const auxRule = "────────────────────────"

func renderAuxiliary(rows []AuxRow) string {
	out := []string{theme.Muted.Render(auxRule)}
	for _, r := range rows {
		icon := r.Icon
		if icon == "" {
			icon = " "
		}
		line := lipgloss.NewStyle().Foreground(theme.ColCyan).Render(icon) +
			" " + theme.Body.Render(r.Label)
		if r.Key != "" {
			line += "  " + theme.Muted.Render("["+r.Key+"]")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
