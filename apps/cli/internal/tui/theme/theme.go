// Package theme is the single source of truth for color tokens and
// pre-built lipgloss styles used by the dots TUI. Every renderable
// across internal/tui must consume these symbols — never an inline
// lipgloss.Color or NewStyle. New tokens require a DESIGN.md update
// in the same commit.
//
// The palette is tokyonight, matching the projection in Ghostty/Zellij
// so the workspace renders coherently. The role names mirror the
// DESIGN.md frontmatter so the contract and the code agree by
// construction.
package theme

import "github.com/charmbracelet/lipgloss"

// Color tokens. Hex values are duplicated in DESIGN.md's `colors:`
// block; if you change one, change the other in the same commit.
const (
	ColBg      lipgloss.Color = "#1a1b26" // surface.base — declared, not painted
	ColFg      lipgloss.Color = "#c0caf5" // text.primary
	ColMuted   lipgloss.Color = "#565f89" // text.muted
	ColAccent  lipgloss.Color = "#7aa2f7" // accent.primary — focal action
	ColTitle   lipgloss.Color = "#bb9af7" // accent.title — section titles
	ColCyan    lipgloss.Color = "#7dcfff" // accent.secondary — keybind hints
	ColSuccess lipgloss.Color = "#9ece6a" // status.success
	ColWarn    lipgloss.Color = "#e0af68" // status.warn
	ColError   lipgloss.Color = "#f7768e" // status.error
)

// Glyph constants. Strings are duplicated where they appear in
// DESIGN.md's component tokens; same single-source-of-truth rule.
const (
	GlyphStepDone    = "✓"
	GlyphStepCurrent = "●"
	GlyphCursor      = "▸"
	GlyphChecked     = "[x]"
	GlyphUnchecked   = "[ ]"
	GlyphMoreAbove   = "▲"
	GlyphMoreBelow   = "▼"
	GlyphLoading     = "◐"
	GlyphBadgeOK     = "✓"
	GlyphBadgeFail   = "✗"
	GlyphBadgeWarn   = "○"
)

// Style helpers. Built once, reused everywhere; views must not call
// NewStyle() inline — extend a helper if a one-off variation is needed.
var (
	Title = lipgloss.NewStyle().Bold(true).Foreground(ColTitle)

	Subtitle = lipgloss.NewStyle().Foreground(ColCyan).Bold(true)

	Body = lipgloss.NewStyle().Foreground(ColFg)

	Muted = lipgloss.NewStyle().Foreground(ColMuted)

	Success = lipgloss.NewStyle().Foreground(ColSuccess).Bold(true)

	Warn = lipgloss.NewStyle().Foreground(ColWarn).Bold(true)

	Error = lipgloss.NewStyle().Foreground(ColError).Bold(true)

	Accent = lipgloss.NewStyle().Foreground(ColAccent).Bold(true)

	Spinner = lipgloss.NewStyle().Foreground(ColAccent)

	// OutcomePanel is the rounded border reserved for outcome surfaces:
	// done, failed, doctor. Wizard step screens are borderless — the
	// stepper, headings, and whitespace carry the structure.
	OutcomePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColMuted).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	// KeyLabel is the [Enter]/[Esc]-style footer key tag.
	KeyLabel = lipgloss.NewStyle().Foreground(ColCyan).Bold(true)

	BadgeOK   = lipgloss.NewStyle().Foreground(ColSuccess).SetString(GlyphBadgeOK)
	BadgeFail = lipgloss.NewStyle().Foreground(ColError).SetString(GlyphBadgeFail)
	BadgeWarn = lipgloss.NewStyle().Foreground(ColWarn).SetString(GlyphBadgeWarn)
)
