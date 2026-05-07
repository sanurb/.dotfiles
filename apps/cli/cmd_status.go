package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/applied"
	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/envelope"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

const cmdStatusSummary = "Show profile, workspace, and last-applied receipt"

// statusProfileJSON is the JSON projection of state.State's
// user-visible fields. Kept in this file because it is specific to the
// status verb's output shape; capture has its own equivalent.
type statusProfileJSON struct {
	Shell       string `json:"shell"`
	Terminal    string `json:"terminal"`
	Multiplexer string `json:"multiplexer"`
	Editor      bool   `json:"editor"`
	Font        bool   `json:"font"`
}

// statusLastApplyJSON is the JSON projection of applied.State. Times
// are emitted in RFC3339 to match what `applied` writes on disk.
type statusLastApplyJSON struct {
	PlanHash  string `json:"planHash"`
	Profile   string `json:"profile"`
	AppliedAt string `json:"appliedAt"`
}

// statusDocJSON is the verb-specific result body nested under
// envelope.Envelope.Result. The pointer fields signal "this host has
// nothing here yet" with a JSON null, which is unambiguous and
// trivial for jq consumers to branch on.
type statusDocJSON struct {
	Workspace string               `json:"workspace"`
	Profile   *statusProfileJSON   `json:"profile"`
	LastApply *statusLastApplyJSON `json:"lastApply"`
	Drift     *statusDriftJSON     `json:"drift"`
}

// statusDriftJSON reports whether the live receipt matches a fresh
// plan computed against the current workspace. Drift is the cheap
// signal — actual closure-level diffing requires a build, which
// status declines to do. Pointers to the right next-step verb live
// in the human renderer; JSON consumers branch on Kind.
type statusDriftJSON struct {
	// Kind is one of "converged", "stale", "no-receipt", "rollback",
	// "unknown" (plan computation failed; e.g., no workspace).
	Kind        string `json:"kind"`
	FreshHash   string `json:"freshHash,omitempty"`
	AppliedHash string `json:"appliedHash,omitempty"`
}

// driftKind is the typed verdict from comparing applied.toml against
// a freshly computed plan. Order is intentional — status renders the
// kinds left-to-right by ascending severity.
type driftKind int

const (
	driftUnknown   driftKind = iota // could not compute a fresh plan
	driftConverged                  // applied hash matches fresh hash
	driftRollback                   // applied via `dots rollback`; receipt has no plan hash
	driftNoReceipt                  // never applied on this host
	driftStale                      // applied hash != fresh hash
)

func (k driftKind) String() string {
	switch k {
	case driftConverged:
		return "converged"
	case driftRollback:
		return "rollback"
	case driftNoReceipt:
		return "no-receipt"
	case driftStale:
		return "stale"
	default:
		return "unknown"
	}
}

// classifyDrift assembles the verdict from the inputs status already
// has on hand. Pure: same inputs, same kind, regardless of host.
func classifyDrift(last *statusLastApplyJSON, freshHash string, freshErr error) driftKind {
	if freshErr != nil {
		return driftUnknown
	}
	if last == nil {
		return driftNoReceipt
	}
	if last.PlanHash == "" {
		return driftRollback
	}
	if last.PlanHash == freshHash {
		return driftConverged
	}
	return driftStale
}

// runStatus implements `dots status [--json]`. Read-only inspector;
// must never mutate. Outside a workspace it exits 0 with a stderr note,
// because "nothing to inspect" is informational, not a usage error.
func runStatus(rest []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	var common cliflags.Common
	common.Bind(fs)
	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	root, werr := workspace.Root()
	if werr != nil {
		// Status is informational. Polite stderr note, exit 0.
		// Exit 2 here would surprise scripts that poll status to
		// answer "is dots installed on this host?" — the answer is
		// "yes, but nothing to report" rather than a usage error.
		if common.JSON {
			_ = envelope.OK(os.Stdout, statusCommandLine(rest), statusDocJSON{}, statusNoWorkspaceActions())
		} else {
			fmt.Fprintln(os.Stderr, "status: no workspace; nothing to report")
		}
		return exitcode.Success
	}

	// Profile: load .dots-state.toml. Validate before returning so we
	// don't print obviously-corrupt values; on validation error we
	// surface it but still emit the rest of the report.
	var profilePtr *statusProfileJSON
	var profileErr error
	if s, found, err := state.Load(state.Path(root)); err != nil {
		profileErr = err
	} else if found {
		profilePtr = &statusProfileJSON{
			Shell:       s.Pillars.Shell,
			Terminal:    s.Pillars.Terminal,
			Multiplexer: s.Pillars.Multiplexer,
			Editor:      s.Capabilities.Editor,
			Font:        s.Capabilities.Font,
		}
	}

	// Last apply: read from the XDG-rooted applied.toml.
	var lastPtr *statusLastApplyJSON
	var appliedErr error
	if path, err := applied.DefaultPath(); err == nil {
		if a, found, lerr := applied.Load(path); lerr != nil {
			appliedErr = lerr
		} else if found {
			lastPtr = &statusLastApplyJSON{
				PlanHash:  a.PlanHash,
				Profile:   a.Profile,
				AppliedAt: a.AppliedAt.UTC().Format(time.RFC3339),
			}
		}
	} else {
		appliedErr = err
	}

	// Drift: compare applied receipt against a freshly computed plan.
	// computePlan failure (e.g., $HOME unreadable) maps to driftUnknown
	// so status stays informational — drift is supplementary, not
	// load-bearing.
	//
	// ConvergedHash (not Hash) is the right signal here: prerequisite
	// steps (snapshot, bootstrap, clone) appear in the work plan but
	// don't describe the desired state. Comparing full Hashes flips to
	// "stale" the moment a snapshot/bootstrap completes — the very
	// thing apply just resolved.
	freshPlan, freshErr := computePlan("")
	freshHash := ""
	if freshErr == nil {
		freshHash = freshPlan.ConvergedHash()
	}
	drift := classifyDrift(lastPtr, freshHash, freshErr)
	driftPtr := &statusDriftJSON{
		Kind:      drift.String(),
		FreshHash: freshHash,
	}
	if lastPtr != nil {
		driftPtr.AppliedHash = lastPtr.PlanHash
	}

	if common.JSON {
		body := statusDocJSON{
			Workspace: root,
			Profile:   profilePtr,
			LastApply: lastPtr,
			Drift:     driftPtr,
		}
		_ = envelope.OK(os.Stdout, statusCommandLine(rest), body, statusActions(drift, root))
		return exitcode.Success
	}

	renderStatusHuman(root, profilePtr, lastPtr, profileErr, appliedErr, drift, freshHash)
	return exitcode.Success
}

// statusCommandLine reconstructs the as-invoked command for the
// envelope's `command` field. Snapshot verbs render this verbatim so
// agents can correlate the response with the request without keeping
// their own bookkeeping.
func statusCommandLine(rest []string) string {
	if len(rest) == 0 {
		return "dots status"
	}
	return "dots status " + joinArgs(rest)
}

// statusNoWorkspaceActions are the next-step affordances when status
// runs outside a workspace. The single load-bearing action is
// `dots init` — the agent's recovery path is to clone or run-via-nix.
func statusNoWorkspaceActions() []envelope.Action {
	return []envelope.Action{
		{
			Command:     "dots init",
			Description: "Clone the dotfiles workspace and run the install wizard.",
		},
	}
}

// statusActions returns the contextual next_actions for a status
// success envelope. Drift drives the urgency: stale or no-receipt
// makes `dots apply` the primary affordance; converged makes
// `dots doctor` the audit.
func statusActions(drift driftKind, _ string) []envelope.Action {
	switch drift {
	case driftStale, driftNoReceipt:
		return []envelope.Action{
			{Command: "dots apply", Description: "Realize the current plan."},
			{Command: "dots plan", Description: "Preview what apply would do."},
		}
	case driftRollback:
		return []envelope.Action{
			{Command: "dots apply", Description: "Re-converge against the current workspace."},
		}
	default: // converged or unknown
		return []envelope.Action{
			{Command: "dots doctor", Description: "Audit the realized environment against the declared persona."},
			{Command: "dots plan", Description: "Preview the next apply."},
		}
	}
}

// renderStatusHuman prints the two-column layout for human consumers.
// stdout for the report itself; stderr for the diagnostic notes about
// missing or malformed inputs so a `dots status | grep` doesn't pick
// up parse-error noise.
func renderStatusHuman(
	root string,
	profile *statusProfileJSON,
	last *statusLastApplyJSON,
	profileErr, appliedErr error,
	drift driftKind,
	freshHash string,
) {
	const labelWidth = "%-11s %s\n"
	fmt.Printf(labelWidth, "workspace", root)

	switch {
	case profileErr != nil:
		fmt.Printf(labelWidth, "profile", "(unreadable)")
		fmt.Fprintf(os.Stderr, "status: read profile: %v\n", profileErr)
	case profile == nil:
		fmt.Printf(labelWidth, "profile", "(no .dots-state.toml — run `dots install`)")
	default:
		caps := capabilitiesSummary(profile.Editor, profile.Font)
		line := fmt.Sprintf("%s · %s · %s%s",
			profile.Shell, profile.Terminal, profile.Multiplexer, caps)
		fmt.Printf(labelWidth, "profile", line)
	}

	switch {
	case appliedErr != nil:
		fmt.Printf(labelWidth, "applied", "(unreadable)")
		fmt.Fprintf(os.Stderr, "status: read applied receipt: %v\n", appliedErr)
	case last == nil:
		fmt.Printf(labelWidth, "applied", "never applied")
	default:
		profileLabel := last.Profile
		if profileLabel == "" {
			profileLabel = "-"
		}
		fmt.Printf(labelWidth, "applied",
			fmt.Sprintf("%s (plan %s) profile %s",
				last.AppliedAt, short(last.PlanHash), profileLabel))
	}

	fmt.Printf(labelWidth, "drift", driftLine(drift, last, freshHash))
}

// driftLine renders the drift verdict as a single line. The wording
// names the next-step verb so a user reading status knows what to do.
func driftLine(kind driftKind, last *statusLastApplyJSON, freshHash string) string {
	switch kind {
	case driftConverged:
		return fmt.Sprintf("converged (plan %s)", short(freshHash))
	case driftStale:
		return fmt.Sprintf("stale: applied %s, fresh would be %s — run `dots apply`",
			short(last.PlanHash), short(freshHash))
	case driftNoReceipt:
		if freshHash != "" {
			return fmt.Sprintf("no receipt — run `dots apply` to land plan %s", short(freshHash))
		}
		return "no receipt — system has never been applied"
	case driftRollback:
		return fmt.Sprintf("rollback in effect; fresh plan would be %s", short(freshHash))
	default:
		return "unknown (could not compute fresh plan)"
	}
}

// capabilitiesSummary renders an inline parenthetical describing which
// capability flags are on — the human row reads as
// "fish · ghostty · zellij  (font, editor)" when both are true. An
// empty string is returned when nothing is enabled, so the line stays
// clean. Git is omitted because git is mandatory infrastructure, not a
// user-toggleable capability — home.nix imports it unconditionally.
func capabilitiesSummary(editor, font bool) string {
	var parts []string
	if font {
		parts = append(parts, "font")
	}
	if editor {
		parts = append(parts, "editor")
	}
	if len(parts) == 0 {
		return ""
	}
	out := "  ("
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out + ")"
}
