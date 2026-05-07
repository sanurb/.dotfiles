package envelope

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestStreamGoldenTranscript pins the full NDJSON shape end-to-end:
// start → step started → step completed → terminal envelope. The
// clock is injected so timestamps are deterministic; the goldens
// also catch field-name drift since we deserialize back into Go
// structs and assert on their fields.
func TestStreamGoldenTranscript(t *testing.T) {
	var buf bytes.Buffer
	t0 := time.Date(2026, 5, 7, 2, 30, 0, 0, time.UTC)
	tick := 0
	clock := func() time.Time {
		t := t0.Add(time.Duration(tick) * 100 * time.Millisecond)
		tick++
		return t
	}

	s := NewStream(&buf, "dots apply", "01HV2K3X9Y4Z5A6B7C8D9E0F1G",
		"/tmp/dots_logs/01HV2K3X9Y4Z5A6B7C8D9E0F1G/apply.log", clock)

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.StepStarted("apply-profile"); err != nil {
		t.Fatalf("StepStarted: %v", err)
	}
	if err := s.StepCompleted("apply-profile"); err != nil {
		t.Fatalf("StepCompleted: %v", err)
	}
	if err := s.Success(map[string]any{"steps_executed": 1}, nil); err != nil {
		t.Fatalf("Success: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), buf.String())
	}

	// Line 1: start event
	var start map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &start); err != nil {
		t.Fatalf("line 1: %v", err)
	}
	if start["type"] != "start" {
		t.Errorf("line 1 type = %v, want start", start["type"])
	}
	if start["command"] != "dots apply" {
		t.Errorf("line 1 command = %v, want dots apply", start["command"])
	}

	// Line 2: step started
	var stepA map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &stepA); err != nil {
		t.Fatalf("line 2: %v", err)
	}
	if stepA["type"] != "step" || stepA["status"] != "started" || stepA["name"] != "apply-profile" {
		t.Errorf("line 2 wrong shape: %v", stepA)
	}
	// Started event should not include duration.
	if _, ok := stepA["duration_ms"]; ok {
		t.Errorf("started event must not include duration_ms")
	}

	// Line 3: step completed with duration_ms = 100 (one clock tick)
	var stepB map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &stepB); err != nil {
		t.Fatalf("line 3: %v", err)
	}
	if stepB["status"] != "completed" {
		t.Errorf("line 3 status = %v, want completed", stepB["status"])
	}
	if got := int64(stepB["duration_ms"].(float64)); got != 100 {
		t.Errorf("duration_ms = %d, want 100 (one 100ms tick)", got)
	}

	// Line 4: terminal envelope (no `type` field — that's the discriminator)
	var terminal map[string]any
	if err := json.Unmarshal([]byte(lines[3]), &terminal); err != nil {
		t.Fatalf("line 4: %v", err)
	}
	if _, ok := terminal["type"]; ok {
		t.Errorf("terminal envelope must not include type field; got %v", terminal["type"])
	}
	if terminal["ok"] != true {
		t.Errorf("terminal ok = %v, want true", terminal["ok"])
	}
	if terminal["run_id"] != "01HV2K3X9Y4Z5A6B7C8D9E0F1G" {
		t.Errorf("terminal run_id wrong: %v", terminal["run_id"])
	}
}

// TestStreamFailureCarriesRunIDAndLogPath pins that the streamer
// auto-attaches its run_id and log_path to a Problem on Failure().
// This is the load-bearing convenience that lets verbs construct a
// bare Problem without threading those fields through every error
// path.
func TestStreamFailureCarriesRunIDAndLogPath(t *testing.T) {
	var buf bytes.Buffer
	s := NewStream(&buf, "dots apply", "01HV", "/tmp/log", nil)
	if err := s.Failure(New(CodeBuildFailed, "boom")); err != nil {
		t.Fatalf("Failure: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw["run_id"] != "01HV" {
		t.Errorf("run_id = %v, want 01HV", raw["run_id"])
	}
	if raw["log_path"] != "/tmp/log" {
		t.Errorf("log_path = %v, want /tmp/log", raw["log_path"])
	}
	errBody := raw["error"].(map[string]any)
	if errBody["code"] != "BUILD_FAILED" {
		t.Errorf("error.code = %v, want BUILD_FAILED", errBody["code"])
	}
}

// TestStepFailedEmitsDuration pins that a failed step still records
// elapsed time. Without it, the failure event would lack the
// "this took 30 seconds before it died" signal agents use to
// distinguish quick rejects from long crashes.
func TestStepFailedEmitsDuration(t *testing.T) {
	var buf bytes.Buffer
	t0 := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	tick := 0
	clock := func() time.Time {
		t := t0.Add(time.Duration(tick) * 250 * time.Millisecond)
		tick++
		return t
	}
	s := NewStream(&buf, "dots apply", "01HV", "/tmp/log", clock)
	_ = s.StepStarted("apply-profile")
	_ = s.StepFailed("apply-profile")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	var failed map[string]any
	_ = json.Unmarshal([]byte(lines[1]), &failed)
	if failed["status"] != "failed" {
		t.Fatalf("status = %v, want failed", failed["status"])
	}
	if got := int64(failed["duration_ms"].(float64)); got != 250 {
		t.Errorf("duration_ms = %d, want 250", got)
	}
}
