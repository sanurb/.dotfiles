// Package exitcode is the single source of truth for dots exit codes.
// The numeric values are part of the CLI's stable contract — scripts
// branch on them. Do not renumber lightly.
package exitcode

const (
	Success   = 0   // success
	Failure   = 1   // generic runtime failure
	Misuse    = 2   // usage error: unknown verb, bad flag, prompt under --non-interactive
	Declined  = 3   // user declined a confirmation (distinct from error)
	PreFlight = 4   // pre-flight failed: doctor drift, unmet prereq, schema mismatch
	NoOp      = 5   // nothing to do (already converged)
	Aborted   = 130 // SIGINT / wizard abort
)
