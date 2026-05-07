package main

import (
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
)

// joinArgs reconstructs an as-invoked argument list for the envelope's
// `command` field. Quoting is intentionally minimal: the field is a
// breadcrumb for agent telemetry, not a re-execable shell string.
// Callers concerned with shell-safe replay should compose the command
// from the result body's typed fields instead.
func joinArgs(rest []string) string {
	if len(rest) == 0 {
		return ""
	}
	return strings.Join(rest, " ")
}

// initCommandLine renders the as-invoked `dots init ...` for the
// envelope's command field. Args parse before the workspace probe
// so we have the full rest slice at every error site.
func initCommandLine(rest []string, _ cliflags.Common) string {
	if len(rest) == 0 {
		return "dots init"
	}
	return "dots init " + strings.Join(rest, " ")
}
