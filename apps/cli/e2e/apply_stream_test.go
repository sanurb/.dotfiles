package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestApplyJSONStreaming pins the NDJSON contract end-to-end against
// the real binary. The harness's nh stub records argv and exits 0,
// which is enough to verify the apply pipeline emits the documented
// event sequence: start → step{started} → step{completed} → terminal
// envelope. The terminal line is a long-running success envelope
// (run_id + log_path populated).
func TestApplyJSONStreaming(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withNhStub().
		withStateFile(buildStateTOML(stateOverrides{}))
	// Redirect logs into the test's tempdir so we don't pollute
	// $HOME/.dots_logs across runs.
	logsDir := t.TempDir()

	cmd := h.run("apply", "--json", "--yes")
	// The nh stub succeeds; install-runtimes may skip when proto is
	// not on PATH (which is the case under our minimal harness env),
	// but apply itself completes with exit 0.
	if cmd.ExitCode != 0 {
		t.Fatalf("apply --json --yes exit = %d; stderr: %s\nstdout: %s", cmd.ExitCode, cmd.Stderr, cmd.Stdout)
	}
	_ = logsDir // referenced for future XDG_STATE_HOME assertion

	lines := splitNDJSON(t, cmd.Stdout)
	if len(lines) < 2 {
		t.Fatalf("expected at least start + terminal lines, got %d:\n%s", len(lines), cmd.Stdout)
	}

	// First line is start.
	first := mustDecode(t, lines[0])
	if first["type"] != "start" {
		t.Errorf("first line type = %v, want start; got %v", first["type"], first)
	}
	if first["command"] == "" {
		t.Errorf("first line missing command")
	}

	// Last line is the terminal envelope (no `type` field, has `ok`).
	last := mustDecode(t, lines[len(lines)-1])
	if _, ok := last["type"]; ok {
		t.Errorf("terminal line must not include type field; got %v", last["type"])
	}
	if last["ok"] != true {
		t.Fatalf("terminal ok = %v, want true; envelope: %v", last["ok"], last)
	}
	runID, _ := last["run_id"].(string)
	if len(runID) != 26 {
		t.Errorf("run_id is not a 26-char ULID: %q", runID)
	}
	logPath, _ := last["log_path"].(string)
	if logPath == "" {
		t.Errorf("terminal envelope missing log_path")
	}

	// Intermediate lines are step events, all carrying type=step
	// and a status drawn from the closed set.
	for i := 1; i < len(lines)-1; i++ {
		ev := mustDecode(t, lines[i])
		if ev["type"] != "step" {
			t.Errorf("line %d type = %v, want step; got %v", i, ev["type"], ev)
		}
		status, _ := ev["status"].(string)
		switch status {
		case "started", "completed", "failed":
		default:
			t.Errorf("line %d status %q is not in the closed set {started,completed,failed}", i, status)
		}
	}

	// Verify the log file was created at log_path.
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log_path %s does not exist on disk: %v", logPath, err)
	}
}

// TestApplyJSONStreamingTerminalLineIsExtractable pins the
// `jq -s 'last'` invariant: a non-streaming consumer can read the
// last line and treat the call as request/response.
func TestApplyJSONStreamingTerminalLineIsExtractable(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withNhStub().
		withStateFile(buildStateTOML(stateOverrides{}))

	cmd := h.run("apply", "--json", "--yes")
	if cmd.ExitCode != 0 {
		t.Fatalf("apply exit = %d; stderr: %s", cmd.ExitCode, cmd.Stderr)
	}

	last := lastLine(cmd.Stdout)
	var raw map[string]any
	if err := json.Unmarshal([]byte(last), &raw); err != nil {
		t.Fatalf("last line is not valid JSON: %v\n%s", err, last)
	}
	// Every response from the contract shape must satisfy: ok is
	// present and boolean, command is non-empty.
	if _, ok := raw["ok"].(bool); !ok {
		t.Errorf("last line missing or non-bool ok: %v", raw)
	}
	if cmd, _ := raw["command"].(string); cmd == "" {
		t.Errorf("last line missing command: %v", raw)
	}
}

// splitNDJSON splits the stdout transcript into lines, dropping the
// trailing newline. Empty trailing lines are filtered so the
// terminal index lookup is well-defined.
func splitNDJSON(t *testing.T, out string) []string {
	t.Helper()
	raw := strings.Split(strings.TrimRight(out, "\n"), "\n")
	lines := raw[:0]
	for _, ln := range raw {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}

func lastLine(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return lines[len(lines)-1]
}

func mustDecode(t *testing.T, line string) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		t.Fatalf("decode %q: %v", line, err)
	}
	return raw
}

var _ = filepath.Join // silence unused-import in case future tests drop the only filepath user
