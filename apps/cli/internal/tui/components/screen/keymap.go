package screen

import (
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/tui/theme"
)

// Keymap is the canonical keybind contract advertised in the footer of
// every wizard screen. Empty fields are omitted from the footer; the
// SelectVerb override lets a confirm screen render "[Enter] confirm"
// instead of "[Enter] select" without reordering slots.
//
// Footer format follows the screenshot:
//
//	↑/k up • ↓/j down • [Enter] select • [Esc] back
//
// Move/Down render as the literal arrow+vim form; named keys render as
// "[<key>]". The separator is " • " in muted color, key labels in
// accent.secondary, descriptions in muted.
type Keymap struct {
	Up         string // e.g. "↑/k"
	Down       string // e.g. "↓/j"
	Select     string // e.g. "Enter"
	Back       string // e.g. "Esc"; "" on the first step
	Quit       string // e.g. "Ctrl-C"; "" to omit
	SelectVerb string // overrides "select", e.g. "confirm"
	// Mnemonics are screen-local affordances bound to auxiliary rows.
	// Each entry is rendered as "[<key>] <label>"; placement follows
	// the standard slots.
	Mnemonics []KeymapEntry
}

// KeymapEntry is one footer slot beyond the canonical keys.
type KeymapEntry struct {
	Key   string
	Label string
}

// DefaultKeymap returns the conventional bindings. Pass includeBack=
// false on the first step in a flow.
func DefaultKeymap(includeBack bool) Keymap {
	km := Keymap{
		Up:     "↑/k",
		Down:   "↓/j",
		Select: "Enter",
	}
	if includeBack {
		km.Back = "Esc"
	}
	return km
}

// Empty reports whether the keymap renders to no footer at all.
func (k Keymap) Empty() bool {
	if k.Up != "" || k.Down != "" || k.Select != "" || k.Back != "" || k.Quit != "" {
		return false
	}
	return len(k.Mnemonics) == 0
}

// Footer renders the keymap as the muted footer line.
func (k Keymap) Footer() string {
	verb := k.SelectVerb
	if verb == "" {
		verb = "select"
	}

	type slot struct{ label, desc string }
	var slots []slot
	if k.Up != "" {
		slots = append(slots, slot{k.Up, "up"})
	}
	if k.Down != "" {
		slots = append(slots, slot{k.Down, "down"})
	}
	if k.Select != "" {
		slots = append(slots, slot{"[" + k.Select + "]", verb})
	}
	if k.Back != "" {
		slots = append(slots, slot{"[" + k.Back + "]", "back"})
	}
	if k.Quit != "" {
		slots = append(slots, slot{"[" + k.Quit + "]", "quit"})
	}
	for _, m := range k.Mnemonics {
		if m.Key == "" {
			continue
		}
		slots = append(slots, slot{"[" + m.Key + "]", m.Label})
	}

	parts := make([]string, len(slots))
	for i, s := range slots {
		parts[i] = theme.KeyLabel.Render(s.label) + " " + theme.Muted.Render(s.desc)
	}
	return strings.Join(parts, theme.Muted.Render(" • "))
}
