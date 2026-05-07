package envelope

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestSnapshotSuccessShape pins the snapshot success envelope.
// Snapshot commands emit no run_id and no log_path — those are
// long-running concerns. A regression here would mean every snapshot
// command grew unwarranted fields.
func TestSnapshotSuccessShape(t *testing.T) {
	var buf bytes.Buffer
	if err := OK(&buf, "dots status", map[string]string{"profile": "fish"}, []Action{
		{Command: "dots apply", Description: "Realize the profile."},
	}); err != nil {
		t.Fatalf("OK: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("emitted line is not valid JSON: %v\n%s", err, buf.String())
	}

	mustHave(t, raw, "ok", true)
	mustHave(t, raw, "command", "dots status")
	if _, ok := raw["run_id"]; ok {
		t.Errorf("snapshot envelope must not include run_id, got: %v", raw["run_id"])
	}
	if _, ok := raw["log_path"]; ok {
		t.Errorf("snapshot envelope must not include log_path, got: %v", raw["log_path"])
	}
	if _, ok := raw["result"].(map[string]any); !ok {
		t.Errorf("result missing or wrong shape")
	}
	if _, ok := raw["next_actions"].([]any); !ok {
		t.Errorf("next_actions missing or wrong shape")
	}
}

// TestLongRunningSuccessShape pins the long-running variant. run_id
// and log_path must be present; result and next_actions follow the
// same shape as snapshot success.
func TestLongRunningSuccessShape(t *testing.T) {
	var buf bytes.Buffer
	if err := OKLong(&buf, "dots apply",
		"01HV2K3X9Y4Z5A6B7C8D9E0F1G",
		"/tmp/dots_logs/01HV2K3X9Y4Z5A6B7C8D9E0F1G/apply.log",
		map[string]string{"steps": "5"},
		nil,
	); err != nil {
		t.Fatalf("OKLong: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	mustHave(t, raw, "ok", true)
	mustHave(t, raw, "run_id", "01HV2K3X9Y4Z5A6B7C8D9E0F1G")
	mustHave(t, raw, "log_path", "/tmp/dots_logs/01HV2K3X9Y4Z5A6B7C8D9E0F1G/apply.log")
}

// TestErrorEnvelopeShape pins the error contract. retryable and
// user_action_required come from the catalog, not the Problem; fix
// defaults from catalog and is overridden only when Problem.Fix is
// set — a verb-specific remediation wins over the generic guidance.
func TestErrorEnvelopeShape(t *testing.T) {
	var buf bytes.Buffer
	p := Wrap(CodeConfigNotFound, errors.New("config file not found: /missing"))
	if err := Fail(&buf, "dots init --config /missing", p); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	mustHave(t, raw, "ok", false)
	mustHave(t, raw, "command", "dots init --config /missing")

	errBody, ok := raw["error"].(map[string]any)
	if !ok {
		t.Fatalf("error must be an object, got %T", raw["error"])
	}
	mustHave(t, errBody, "message", "config file not found: /missing")
	mustHave(t, errBody, "code", "CONFIG_NOT_FOUND")

	mustHave(t, raw, "retryable", false)
	mustHave(t, raw, "user_action_required", true)

	fix, _ := raw["fix"].(string)
	if fix == "" {
		t.Fatalf("fix must be populated from catalog default; got empty")
	}
}

// TestErrorFixOverride pins that a verb-supplied fix overrides the
// catalog default. The override is the load-bearing path — most real
// errors have context the catalog default can't carry.
func TestErrorFixOverride(t *testing.T) {
	var buf bytes.Buffer
	p := Wrap(CodeConfigNotFound, errors.New("nope")).WithFix("custom remediation")
	if err := Fail(&buf, "dots", p); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if !strings.Contains(buf.String(), `"fix":"custom remediation"`) {
		t.Fatalf("verb-supplied fix must override catalog default; got\n%s", buf.String())
	}
}

// TestUnknownCodeFallsBackToInternal pins the catalog-miss policy:
// emit a valid envelope (under INTERNAL_ERROR's metadata) rather than
// silently corrupting the wire shape. The test enforces wire-shape
// stability under bug conditions.
func TestUnknownCodeFallsBackToInternal(t *testing.T) {
	var buf bytes.Buffer
	p := New(Code("NOT_IN_CATALOG"), "this code was never registered")
	if err := Fail(&buf, "dots foo", p); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// The code field still echoes what the verb passed (so the test
	// surface tells the truth), but the metadata flags follow
	// INTERNAL_ERROR so agents see consistent retryability semantics.
	internalMeta := catalog[CodeInternalError]
	if got := raw["retryable"].(bool); got != internalMeta.Retryable {
		t.Errorf("retryable on unknown code = %v, want %v (INTERNAL_ERROR's value)", got, internalMeta.Retryable)
	}
	if got := raw["user_action_required"].(bool); got != internalMeta.UserActionRequired {
		t.Errorf("user_action_required on unknown code = %v, want %v", got, internalMeta.UserActionRequired)
	}
}

// TestStreamEventShape pins the intermediate-line shape: type and ts
// always; name/status/duration_ms only when applicable. The
// terminal-line discriminator is the absence of a type field, so a
// stream event must always carry one.
func TestStreamEventShape(t *testing.T) {
	var buf bytes.Buffer
	if err := EmitEvent(&buf, StreamEvent{Type: EventStart, Command: "dots apply"}); err != nil {
		t.Fatalf("EmitEvent: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	mustHave(t, raw, "type", "start")
	mustHave(t, raw, "command", "dots apply")
	if _, ok := raw["name"]; ok {
		t.Errorf("start event must not include name; got %v", raw["name"])
	}
}

// TestEmitEmitsExactlyOneLine pins the NDJSON invariant: one event
// per call, terminated by exactly one newline. A streaming consumer
// reads line-by-line; embedded newlines or missing terminators
// corrupt the boundary.
func TestEmitEmitsExactlyOneLine(t *testing.T) {
	var buf bytes.Buffer
	if err := OK(&buf, "x", nil, nil); err != nil {
		t.Fatalf("OK: %v", err)
	}
	s := buf.String()
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("envelope must terminate with newline, got %q", s)
	}
	if strings.Count(s, "\n") != 1 {
		t.Errorf("envelope must contain exactly one newline, got %d in %q", strings.Count(s, "\n"), s)
	}
}

// TestProblemUnwrapPreservesChain pins the wrapping contract:
// errors.Is and errors.As reach the underlying error through Problem,
// so legacy error-checking code keeps working when paths are
// promoted from `return err` to `return envelope.Wrap(...)`.
func TestProblemUnwrapPreservesChain(t *testing.T) {
	sentinel := errors.New("the underlying cause")
	p := Wrap(CodeBuildFailed, sentinel)
	if !errors.Is(p, sentinel) {
		t.Fatalf("errors.Is must reach the wrapped sentinel through Problem")
	}
}

func mustHave(t *testing.T, m map[string]any, key string, want any) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Fatalf("envelope missing %q; got keys %v", key, keys(m))
	}
	if got != want {
		t.Fatalf("envelope[%q] = %v (%T), want %v (%T)", key, got, got, want, want)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
