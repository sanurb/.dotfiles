package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestJSONContract pins the envelope contract end-to-end. Each verb
// is exercised through the real binary against a real workspace; the
// stdout is unmarshalled and asserted against the documented shape.
//
// These tests are the contract guard: any drift between
// docs/cli-json-contract.md, the envelope package, and what a verb
// emits is caught here before users see it.
func TestJSONContract(t *testing.T) {
	t.Run("status --json with workspace and state", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withNhStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		got := h.run("status", "--json")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		assertEqual(t, env["command"], "dots status --json")
		assertEqual(t, env["ok"], true)
		// Snapshot envelopes never carry run_id or log_path.
		assertAbsent(t, env, "run_id")
		assertAbsent(t, env, "log_path")
		// Result body has a workspace path; we just check shape, not the value.
		body := mustObject(t, env, "result")
		if _, ok := body["workspace"].(string); !ok {
			t.Fatalf("result.workspace missing or wrong type: %v", body)
		}
		if _, ok := env["next_actions"].([]any); !ok {
			t.Fatalf("next_actions missing or wrong type")
		}
	})

	t.Run("profile show --json with no profile is a success-with-null", func(t *testing.T) {
		// No state file written; "no profile yet" is informational.
		h := newHarness(t).withStub("nix", nixStubBody)
		got := h.run("profile", "show", "--json")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		assertEqual(t, env["ok"], true)
		assertEqual(t, env["result"], nil)
		// next_actions must point to `dots init` so an agent can recover.
		actions := mustArray(t, env, "next_actions")
		if !actionsContainCommand(actions, "dots init") {
			t.Fatalf("next_actions must include `dots init`, got %v", actions)
		}
	})

	t.Run("profile show --json with a profile renders pillars + caps", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withStateFile(buildStateTOML(stateOverrides{Shell: "zsh"}))

		got := h.run("profile", "show", "--json")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		body := mustObject(t, env, "result")
		assertEqual(t, body["shell"], "zsh")
		// schemaVersion must NOT leak into the envelope; verb-specific
		// schemas live inside content schemas (plan/applied), never on
		// the envelope.
		assertAbsent(t, body, "schemaVersion")
	})

	t.Run("plan --json emits the full plan as result", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withNhStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		got := h.run("plan", "--json")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		body := mustObject(t, env, "result")
		// plan's own SchemaVersion stays on the Plan content schema —
		// it's the contract `dots apply --plan FILE` validates against.
		// That is distinct from the envelope-level schema we cut.
		if _, ok := body["schemaVersion"]; !ok {
			t.Errorf("plan content body must keep schemaVersion (Plan's own contract), got keys %v", keysOf(body))
		}
		if _, ok := body["steps"]; !ok {
			t.Errorf("plan content body must include steps")
		}
	})

	t.Run("plan --out --json emits a summary pointing at the file", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withNhStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		out := filepath.Join(h.Workspace, "plan.json")
		got := h.run("plan", "--out", out, "--json")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		body := mustObject(t, env, "result")
		assertEqual(t, body["out_path"], out)
		// Summary must NOT include the full step list — the file is
		// the single source of truth for replay.
		assertAbsent(t, body, "steps")
		actions := mustArray(t, env, "next_actions")
		if !actionsContainCommand(actions, "dots apply --plan <path>") {
			t.Fatalf("next_actions must template `dots apply --plan <path>`, got %v", actions)
		}
	})

	t.Run("apply --dry-run --json is a snapshot envelope", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withNhStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		got := h.run("apply", "--dry-run", "--json")
		// dry-run with empty plan returns NoOp (5); with steps it's 0.
		// We only care that ExitCode is one of those, not which.
		if got.ExitCode != 0 && got.ExitCode != 5 {
			t.Fatalf("dry-run unexpected exit code %d; stderr: %s", got.ExitCode, got.Stderr)
		}
		env := decodeSuccess(t, got.Stdout)
		assertEqual(t, env["ok"], true)
		// Even though apply IS long-running, --dry-run is point-in-time.
		assertAbsent(t, env, "run_id")
		assertAbsent(t, env, "log_path")
	})

	t.Run("why --json with no path is a usage error envelope", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withStateFile(buildStateTOML(stateOverrides{}))

		got := h.run("why", "--json")
		assertEqual(t, got.ExitCode, 2)

		env := decodeError(t, got.Stdout)
		assertEqual(t, env["ok"], false)
		errBody := mustObject(t, env, "error")
		assertEqual(t, errBody["code"], "INVALID_ARGUMENT")
		assertEqual(t, env["user_action_required"], true)
		assertEqual(t, env["retryable"], false)
		if _, ok := env["fix"].(string); !ok {
			t.Fatalf("fix must be populated; got %v", env["fix"])
		}
	})

	t.Run("why --json on a managed path returns owner + apply action", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withStateFile(buildStateTOML(stateOverrides{}))
		// Plant a fake module that "owns" the queried path.
		mustWrite(h.t, filepath.Join(h.Workspace, "modules/home/foo.nix"),
			`{ ... }: { xdg.configFile."test/file.txt".source = ./x; }`)

		got := h.run("why", "--json", "~/.config/test/file.txt")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		body := mustObject(t, env, "result")
		assertEqual(t, body["status"], "managed")
		actions := mustArray(t, env, "next_actions")
		if !actionsContainCommand(actions, "dots apply") {
			t.Fatalf("managed why must suggest `dots apply`, got %v", actions)
		}
	})

	t.Run("capture --json emits the full doc on stdout", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withStateFile(buildStateTOML(stateOverrides{}))

		got := h.run("capture", "--json")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		body := mustObject(t, env, "result")
		// capture's content schema keeps schemaVersion (it round-trips
		// through `dots init --config` on a peer host).
		if _, ok := body["schemaVersion"]; !ok {
			t.Errorf("capture content body must keep schemaVersion")
		}
	})

	t.Run("capture --output --json emits a summary pointing at the file", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withStateFile(buildStateTOML(stateOverrides{}))

		out := filepath.Join(h.Workspace, "captured.toml")
		got := h.run("capture", "--output", out, "--json")
		assertEqual(t, got.ExitCode, 0)

		env := decodeSuccess(t, got.Stdout)
		body := mustObject(t, env, "result")
		assertEqual(t, body["out_path"], out)
		// Summary must NOT include the full host fingerprint or
		// schemaVersion; it points at the file.
		assertAbsent(t, body, "schemaVersion")
		assertAbsent(t, body, "host")
	})
}

// TestEveryEnvelopeIsValidJSON pins that every JSON-mode invocation
// emits a single line of valid JSON. A prose leak would unmarshal to
// nothing and this test catches it before users do.
func TestEveryEnvelopeIsValidJSON(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withNhStub().
		withStateFile(buildStateTOML(stateOverrides{}))

	verbs := [][]string{
		{"status", "--json"},
		{"profile", "show", "--json"},
		{"plan", "--json"},
		{"apply", "--dry-run", "--json"},
		{"why", "--json", "/tmp/nope"},
		{"capture", "--json"},
	}
	for _, argv := range verbs {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			got := h.run(argv...)
			// Stdout must contain at least one full JSON line.
			lines := strings.Split(strings.TrimRight(got.Stdout, "\n"), "\n")
			if len(lines) == 0 || lines[0] == "" {
				t.Fatalf("no stdout from %v; stderr: %s", argv, got.Stderr)
			}
			for i, ln := range lines {
				var raw any
				if err := json.Unmarshal([]byte(ln), &raw); err != nil {
					t.Fatalf("line %d of %v is not valid JSON: %v\n%s", i, argv, err, ln)
				}
			}
		})
	}
}

// decodeSuccess unmarshals stdout as a single-line success envelope,
// failing the test on any deviation. Multi-line stdout (intermediate
// stream events) makes this fail — which is correct: snapshot verbs
// must never stream.
func decodeSuccess(t *testing.T, stdout string) map[string]any {
	t.Helper()
	stdout = strings.TrimRight(stdout, "\n")
	if strings.Contains(stdout, "\n") {
		t.Fatalf("snapshot stdout must be a single line; got\n%s", stdout)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, stdout)
	}
	if ok, _ := raw["ok"].(bool); !ok {
		t.Fatalf("expected ok:true envelope, got %v", raw)
	}
	return raw
}

func decodeError(t *testing.T, stdout string) map[string]any {
	t.Helper()
	stdout = strings.TrimRight(stdout, "\n")
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("decode error envelope: %v\n%s", err, stdout)
	}
	if ok, _ := raw["ok"].(bool); ok {
		t.Fatalf("expected ok:false envelope, got %v", raw)
	}
	return raw
}

func mustObject(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s missing or not an object: %v", key, parent[key])
	}
	return v
}

func mustArray(t *testing.T, parent map[string]any, key string) []any {
	t.Helper()
	v, ok := parent[key].([]any)
	if !ok {
		t.Fatalf("%s missing or not an array: %v", key, parent[key])
	}
	return v
}

func assertAbsent(t *testing.T, m map[string]any, key string) {
	t.Helper()
	if _, ok := m[key]; ok {
		t.Errorf("%s must be absent (snapshot envelope), got %v", key, m[key])
	}
}

func actionsContainCommand(actions []any, command string) bool {
	for _, a := range actions {
		if obj, ok := a.(map[string]any); ok {
			if obj["command"] == command {
				return true
			}
		}
	}
	return false
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
