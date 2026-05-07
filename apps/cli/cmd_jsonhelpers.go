package main

import (
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
)

// commandLine renders an as-invoked `dots <verb> [args...]` string for
// the envelope's `command` field. Quoting is intentionally minimal:
// the field is a breadcrumb for agent telemetry, not a re-execable
// shell string. Callers concerned with shell-safe replay should
// compose the command from the result body's typed fields instead.
func commandLine(verb string, rest []string) string {
	if len(rest) == 0 {
		return "dots " + verb
	}
	return "dots " + verb + " " + strings.Join(rest, " ")
}

// personaJSON is the JSON projection of state.State's user-visible
// pillars + capabilities. Shared across status, profile show, and
// capture so the wire shape never drifts between verbs that report
// the same persona; the state package speaks TOML on disk only, so
// we maintain JSON tags here.
type personaJSON struct {
	Shell       string `json:"shell"`
	Terminal    string `json:"terminal"`
	Multiplexer string `json:"multiplexer"`
	Editor      bool   `json:"editor"`
	Font        bool   `json:"font"`
}

func personaJSONFromState(s state.State) personaJSON {
	return personaJSON{
		Shell:       s.Pillars.Shell,
		Terminal:    s.Pillars.Terminal,
		Multiplexer: s.Pillars.Multiplexer,
		Editor:      s.Capabilities.Editor,
		Font:        s.Capabilities.Font,
	}
}
