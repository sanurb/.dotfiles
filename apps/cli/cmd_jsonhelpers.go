package main

import "strings"

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
