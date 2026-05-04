package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/applied"
	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
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
	Git         bool   `json:"git"`
}

// statusLastApplyJSON is the JSON projection of applied.State. Times
// are emitted in RFC3339 to match what `applied` writes on disk.
type statusLastApplyJSON struct {
	PlanHash  string `json:"planHash"`
	Profile   string `json:"profile"`
	AppliedAt string `json:"appliedAt"`
}

// statusDocJSON is the full envelope. The pointer fields signal "this
// host has nothing here yet" with a JSON null, which is unambiguous and
// trivial for jq consumers to branch on.
type statusDocJSON struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Workspace     string               `json:"workspace"`
	Profile       *statusProfileJSON   `json:"profile"`
	LastApply     *statusLastApplyJSON `json:"lastApply"`
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
			doc := statusDocJSON{SchemaVersion: 1}
			emitStatusJSON(doc)
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
			Git:         s.Capabilities.Git,
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

	if common.JSON {
		emitStatusJSON(statusDocJSON{
			SchemaVersion: 1,
			Workspace:     root,
			Profile:       profilePtr,
			LastApply:     lastPtr,
		})
		return exitcode.Success
	}

	renderStatusHuman(root, profilePtr, lastPtr, profileErr, appliedErr)
	return exitcode.Success
}

// emitStatusJSON writes the JSON envelope to stdout with two-space
// indent — the same shape every other --json surface uses.
func emitStatusJSON(doc statusDocJSON) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
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
		caps := capabilitiesSummary(profile.Editor, profile.Git)
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
		hash := last.PlanHash
		if len(hash) > 8 {
			hash = hash[:8]
		}
		profileLabel := last.Profile
		if profileLabel == "" {
			profileLabel = "-"
		}
		fmt.Printf(labelWidth, "applied",
			fmt.Sprintf("%s (plan %s) profile %s",
				last.AppliedAt, hash, profileLabel))
	}
}

// capabilitiesSummary renders an inline parenthetical describing which
// capability flags are on — the human row reads as
// "fish · ghostty · zellij  (editor, git)" when both are true. An empty
// string is returned when nothing is enabled, so the line stays clean.
func capabilitiesSummary(editor, git bool) string {
	var parts []string
	if editor {
		parts = append(parts, "editor")
	}
	if git {
		parts = append(parts, "git")
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
