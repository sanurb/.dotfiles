package ui

import "github.com/charmbracelet/lipgloss"

// Single source of truth for the 2026 color scheme. Aligns with the
// tokyonight palette already projected into Ghostty/Zellij. Every
// renderable in this package MUST style through one of these constants
// or the styBy* helpers — no inline lipgloss.Color in views.
const (
	colBg      lipgloss.Color = "#1a1b26"
	colFg      lipgloss.Color = "#c0caf5"
	colMuted   lipgloss.Color = "#565f89"
	colAccent  lipgloss.Color = "#7aa2f7" // primary action
	colMagenta lipgloss.Color = "#bb9af7" // titles
	colSuccess lipgloss.Color = "#9ece6a"
	colWarn    lipgloss.Color = "#e0af68"
	colError   lipgloss.Color = "#f7768e"
	colCyan    lipgloss.Color = "#7dcfff"
)

var (
	styTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colMagenta).
			MarginBottom(1)

	stySubtitle = lipgloss.NewStyle().
			Foreground(colCyan).
			Bold(true)

	styBody = lipgloss.NewStyle().
		Foreground(colFg)

	styMuted = lipgloss.NewStyle().
			Foreground(colMuted)

	stySuccess = lipgloss.NewStyle().
			Foreground(colSuccess).
			Bold(true)

	styWarn = lipgloss.NewStyle().
		Foreground(colWarn).
		Bold(true)

	styError = lipgloss.NewStyle().
		Foreground(colError).
		Bold(true)

	stySpinner = lipgloss.NewStyle().
			Foreground(colAccent)

	styPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colMuted).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	styBadgeOK = lipgloss.NewStyle().
			Foreground(colSuccess).
			SetString("✓")

	styBadgeFail = lipgloss.NewStyle().
			Foreground(colError).
			SetString("✗")

	styBadgeWarn = lipgloss.NewStyle().
			Foreground(colWarn).
			SetString("○")

	styKey = lipgloss.NewStyle().
		Foreground(colCyan).
		Bold(true)
)
