package envelope

import (
	"encoding/json"
	"io"
)

// Problem composes a Code with per-occurrence detail. Verbs construct
// a Problem when they hit a known failure mode; Fail then composes
// the wire envelope by joining Problem with the catalog metadata.
//
// Problem implements error so it can wrap stdlib errors transparently
// — a verb's existing error path can be promoted by replacing
// `return err` with `return envelope.Wrap(envelope.CodeX, err)` and
// the rest of the code keeps reading it as an error.
type Problem struct {
	Code        Code
	Message     string // overrides err.Error() when non-empty
	Fix         string // overrides catalog DefaultFix when non-empty
	NextActions []Action
	RunID       string
	LogPath     string

	err error
}

// New constructs a Problem with an explicit message — used when the
// failure originates inside the verb (no underlying error to wrap)
// and the verb already knows the user-facing string.
func New(code Code, message string) *Problem {
	return &Problem{Code: code, Message: message}
}

// Wrap promotes a stdlib error into a Problem. The wrapped error is
// preserved for errors.Is / errors.As; .Error() falls through to the
// wrapped error's text when Message is empty.
func Wrap(code Code, err error) *Problem {
	return &Problem{Code: code, err: err}
}

func (p *Problem) WithFix(fix string) *Problem { p.Fix = fix; return p }

func (p *Problem) WithNextActions(a ...Action) *Problem { p.NextActions = a; return p }

// WithRunID attaches the run_id to a Problem before its surrounding
// Stream exists. Used by the long-running paths' early-failure
// branches (e.g., log file open failed) where there is no stream to
// auto-populate the field.
func (p *Problem) WithRunID(id string) *Problem { p.RunID = id; return p }

func (p *Problem) Error() string {
	if p.Message != "" {
		return p.Message
	}
	if p.err != nil {
		return p.err.Error()
	}
	return string(p.Code)
}

func (p *Problem) Unwrap() error { return p.err }

// emit writes one JSON line + newline to w. Single point of contact
// with stdout under --json; every other path is a contract bug.
// HTML escape is disabled — the JSON is consumed by agents and
// pipelines, never by browsers.
func emit(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// OK emits a snapshot success envelope. Snapshot commands return
// synchronously without producing a per-run log; RunID and LogPath
// are intentionally absent (omitempty).
func OK(w io.Writer, command string, result any, next []Action) error {
	return emit(w, Envelope{
		OK:          true,
		Command:     command,
		Result:      result,
		NextActions: next,
	})
}

// OKLong emits a long-running success envelope: same shape as OK plus
// the run_id and log_path that pair with the NDJSON stream and the
// per-run log file.
func OKLong(w io.Writer, command, runID, logPath string, result any, next []Action) error {
	return emit(w, Envelope{
		OK:          true,
		Command:     command,
		RunID:       runID,
		LogPath:     logPath,
		Result:      result,
		NextActions: next,
	})
}

// Fail emits an error envelope from a Problem. The catalog supplies
// retryable and user_action_required; Problem.Fix overrides the
// catalog default when set (so a verb with specific remediation
// context wins over the generic guidance). Returns the encoder error,
// not p — callers translate the failure back to an exit code.
func Fail(w io.Writer, command string, p *Problem) error {
	meta, ok := Lookup(p.Code)
	if !ok {
		// Catalog miss is a programming error; tests catch it. In
		// production, fall back to INTERNAL_ERROR so the wire shape
		// stays valid rather than corrupting the contract.
		meta = catalog[CodeInternalError]
	}
	fix := p.Fix
	if fix == "" {
		fix = meta.DefaultFix
	}
	msg := p.Message
	if msg == "" && p.err != nil {
		msg = p.err.Error()
	}
	return emit(w, ErrorEnvelope{
		OK:                 false,
		Command:            command,
		RunID:              p.RunID,
		LogPath:            p.LogPath,
		Error:              ErrorBody{Message: msg, Code: string(p.Code)},
		Retryable:          meta.Retryable,
		UserActionRequired: meta.UserActionRequired,
		Fix:                fix,
		NextActions:        p.NextActions,
	})
}

// EmitEvent writes one streaming event line to w. Intermediate-line
// helper paired with OKLong / Fail for the terminal line. The caller
// owns the timestamp policy (real wall clock in production, injected
// for tests) so EmitEvent never reads time.Now itself.
func EmitEvent(w io.Writer, ev StreamEvent) error {
	return emit(w, ev)
}
