// Package envelope is the stable JSON contract every dots --json command
// emits on stdout. It exists so agents (Claude Code, opencode, scripts)
// can classify outcomes, decide whether to retry, find logs, and chain
// commands without parsing prose.
//
// The contract is forever once shipped. Fields are added freely; never
// removed or renamed. There is no schema_version field — the shape
// itself is the contract, and additive evolution is the migration plan.
//
// The package is the single point of stdout contact under --json. Any
// cmd_*.go path that writes JSON-shaped output to stdout outside this
// package is a contract bug; the lint in apps/cli/jsoncontract_test.go
// fails the build if one appears.
package envelope

import "time"

// Envelope is the shape every successful --json command emits as its
// terminal stdout line. RunID and LogPath are populated only by
// long-running commands (apply, sync, update, rollback) where they
// pair with the NDJSON stream and the per-run log file. Snapshot
// commands omit both — there is no run to identify and no log to
// reference.
type Envelope struct {
	OK          bool     `json:"ok"`
	Command     string   `json:"command"`
	RunID       string   `json:"run_id,omitempty"`
	LogPath     string   `json:"log_path,omitempty"`
	Result      any      `json:"result"`
	NextActions []Action `json:"next_actions,omitempty"`
}

// ErrorEnvelope mirrors Envelope for failures.
//
// Code is the agent's fine-grained discriminator (closed catalog in
// catalog.go). Retryable and UserActionRequired are the coarse axes
// most agents branch on:
//
//   - retryable=true  + user_action=false → safe to retry as-is
//   - retryable=true  + user_action=true  → fix something, then retry
//   - retryable=false + user_action=true  → escalate; no auto-recovery
//   - retryable=false + user_action=false → terminal user-initiated
//     decision (declined, aborted) — no retry, no escalation
//
// Fix is plain-language remediation. Agents may execute it; humans
// read it. NextActions are HATEOAS affordances — what you can run
// next, with the placeholders pre-filled when context allows.
type ErrorEnvelope struct {
	OK                 bool      `json:"ok"`
	Command            string    `json:"command"`
	RunID              string    `json:"run_id,omitempty"`
	LogPath            string    `json:"log_path,omitempty"`
	Error              ErrorBody `json:"error"`
	Retryable          bool      `json:"retryable"`
	UserActionRequired bool      `json:"user_action_required"`
	Fix                string    `json:"fix"`
	NextActions        []Action  `json:"next_actions,omitempty"`
}

// ErrorBody nests the per-occurrence message and the catalog code so
// agents can match either structurally (`.error.code == "X"`) or by
// retryability (`.retryable`) without reaching across siblings.
type ErrorBody struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Action is one HATEOAS next-step affordance. Command is a literal
// string when Params is nil; otherwise it is a template with
// POSIX/docopt-style placeholders (`<required>`, `[--flag <value>]`)
// that the agent fills using Params metadata.
type Action struct {
	Command     string                 `json:"command"`
	Description string                 `json:"description"`
	Params      map[string]ActionParam `json:"params,omitempty"`
}

// ActionParam describes one placeholder in an Action.Command template.
// Value pre-fills from current context (the action emitter knows the
// concrete value); Default is the value when the user omits the flag;
// Enum closes the value set; Required marks positional args.
type ActionParam struct {
	Description string   `json:"description,omitempty"`
	Value       string   `json:"value,omitempty"`
	Default     string   `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Required    bool     `json:"required,omitempty"`
}

// StreamEvent is one intermediate line of an NDJSON stream. The
// terminal line is an Envelope or ErrorEnvelope (no Type field) — the
// presence of `ok` discriminates terminal lines from intermediate
// lines. `jq -s 'last'` extracts the terminal envelope for consumers
// that don't care about streaming.
//
// The vocabulary is intentionally minimal: start (stream began) and
// step (phase lifecycle). progress and log events are not emitted in
// v1 — the per-run log file (LogPath on the terminal envelope) is the
// detail surface; the stream only carries phase boundaries.
type StreamEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"ts"`

	// Populated on Type="start".
	Command string `json:"command,omitempty"`

	// Populated on Type="step".
	Name       string `json:"name,omitempty"`
	Status     string `json:"status,omitempty"` // "started" | "completed" | "failed"
	DurationMs int64  `json:"duration_ms,omitempty"`
}

// Stream event Type values. Adding a new event type is additive and
// safe; renaming or removing one is a contract break.
const (
	EventStart = "start"
	EventStep  = "step"
)

// Step status values. Same evolution rules as event types.
const (
	StepStarted   = "started"
	StepCompleted = "completed"
	StepFailed    = "failed"
)
