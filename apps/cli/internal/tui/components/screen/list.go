package screen

import (
	"fmt"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// Row is one selectable line in the option list. Cursor marks the
// currently focused row; Selected marks the row that is the user's
// committed choice (single- or multi-select); Default marks the
// recommended option (the one Enter would pick if the cursor were on
// it). The flags are independent — any combination is legal.
type Row struct {
	Label    string
	Detail   string // optional muted right-side description
	Cursor   bool
	Selected bool
	Default  bool
}

// RenderRows produces the option-list block for Layout.Content. Every
// row rendered through this function inherits the cursor / selection
// contract from DESIGN.md §components.list-item; views must not render
// rows by hand.
func RenderRows(rows []Row) string {
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = renderRow(r)
	}
	return strings.Join(lines, "\n")
}

func renderRow(r Row) string {
	cursor := "  "
	if r.Cursor {
		cursor = theme.Accent.Render(theme.GlyphCursor) + " "
	}

	label := theme.Body.Render(r.Label)
	if r.Selected {
		label = theme.Accent.Render(r.Label)
	}
	if r.Cursor && !r.Selected {
		label = theme.Accent.Render(r.Label)
	}

	out := cursor + label
	if r.Detail != "" {
		out += "  " + theme.Muted.Render(r.Detail)
	}
	if r.Default {
		out += " " + theme.Muted.Render("(default)")
	}
	return out
}

// Viewport renders a list with scroll indicators when the row count
// exceeds the visible height. Cursor is the 0-indexed focused row; a
// Height of 0 disables truncation. Returns the rendered body and 1-
// indexed (pos, total) for an optional position counter.
type Viewport struct {
	Rows   []Row
	Cursor int
	Height int
}

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
		lines = append(lines, theme.Muted.Render(fmt.Sprintf("%s %d more above", theme.GlyphMoreAbove, start)))
	}
	for i := start; i < end; i++ {
		lines = append(lines, renderRow(v.Rows[i]))
	}
	if end < total {
		lines = append(lines, theme.Muted.Render(fmt.Sprintf("%s %d more below", theme.GlyphMoreBelow, total-end)))
	}
	return strings.Join(lines, "\n"), pos, total
}
