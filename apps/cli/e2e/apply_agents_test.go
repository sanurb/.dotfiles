package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// dummyDigest is a syntactically valid SHA-256 that no download ever
// checks against. Tests here either converge without downloading or
// fail before any network I/O, so a real digest would only make the
// fixture look like it means something.
const dummyDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// withAgentsManifest plants config/agents/agents.toml in the harness
// workspace. Its presence is what makes computePlan emit the
// sync-agents step, so tests that omit it exercise the pre-plane
// behavior unchanged.
func (h *harness) withAgentsManifest(toml string) *harness {
	h.t.Helper()
	mustWrite(h.t, filepath.Join(h.Workspace, "config", "agents", "agents.toml"), toml)
	return h
}

// withAgentStub plants a fake agent binary on the front of PATH that
// answers --version with the given line. This is how a test decides
// whether the roster reads as converged or drifted.
func (h *harness) withAgentStub(name, versionLine string) *harness {
	return h.withStub(name, "#!/bin/sh\necho '"+versionLine+"'\nexit 0\n")
}

// agentsManifest builds a one-row github-release manifest whose digest
// map covers only the given target. Passing a target no host can be
// makes convergence fail before it touches the network.
func agentsManifest(version, digestTarget string) string {
	return `schema_version = 1

[agents.herdr]
provider = "github-release"
version  = "` + version + `"
repo     = "herdrdev/herdr"
asset    = "herdr-{os}-{arch}"
probe    = "herdr --version"

[agents.herdr.sha256]
` + digestTarget + ` = "` + dummyDigest + `"
`
}

// stepEvents returns the names of every step event in an NDJSON stream
// that reached the given status.
func stepEvents(t *testing.T, stdout, status string) []string {
	t.Helper()
	var out []string
	for _, line := range splitNDJSON(t, stdout) {
		ev := mustDecode(t, line)
		if ev["type"] == "step" && ev["status"] == status {
			name, _ := ev["name"].(string)
			out = append(out, name)
		}
	}
	return out
}

// A converged roster must let apply proceed all the way to activation.
// The stub agent reports exactly what the manifest pins, so the step
// probes, finds nothing to do, and completes.
func TestApplySyncsAgentsThenActivates(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withNhStub().
		withStateFile(buildStateTOML(stateOverrides{})).
		withAgentsManifest(agentsManifest("0.8.2", "linux-x86_64")).
		withAgentStub("herdr", "herdr 0.8.2")

	cmd := h.run("apply", "--json", "--yes")
	if cmd.ExitCode != 0 {
		t.Fatalf("apply exit = %d; stdout: %s\nstderr: %s", cmd.ExitCode, cmd.Stdout, cmd.Stderr)
	}
	completed := stepEvents(t, cmd.Stdout, "completed")
	if !contains(completed, "sync-agents") {
		t.Errorf("completed steps = %v, want sync-agents among them", completed)
	}
	if !contains(completed, "apply-profile") {
		t.Errorf("completed steps = %v, want apply-profile among them", completed)
	}
	if _, ran := h.nhArgs(); !ran {
		t.Error("nh stub never ran; activation did not happen")
	}
}

// The ordering guarantee, asserted the only way that actually proves
// it: an agent that cannot be installed must stop the apply *before*
// nh runs. herdr's activation hook installs plugins through the binary
// this step provides, so activating over a failed sync would run the
// hook against a plane that was never converged.
func TestApplyStopsBeforeActivationWhenAgentSyncFails(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withNhStub().
		withStateFile(buildStateTOML(stateOverrides{})).
		// Digest recorded only for a platform no host can be, so the
		// converge fails before any network I/O.
		withAgentsManifest(agentsManifest("0.8.2", "solaris-sparc")).
		// Drifted: the stub reports an older version than the manifest
		// pins, so the step has real work to do.
		withAgentStub("herdr", "herdr 0.1.0")

	cmd := h.run("apply", "--json", "--yes")
	if cmd.ExitCode == 0 {
		t.Fatalf("apply exit = 0; an uninstallable agent must fail the run\nstdout: %s", cmd.Stdout)
	}

	lines := splitNDJSON(t, cmd.Stdout)
	last := mustDecode(t, lines[len(lines)-1])
	if last["ok"] != false {
		t.Fatalf("terminal envelope ok = %v, want false: %v", last["ok"], last)
	}
	errBody, _ := last["error"].(map[string]any)
	if errBody["code"] != "AGENT_SYNC_FAILED" {
		t.Errorf("error.code = %v, want AGENT_SYNC_FAILED", errBody["code"])
	}
	if fix, _ := last["fix"].(string); !strings.Contains(fix, "--skip-agents") {
		t.Errorf("fix = %q, want it to name the --skip-agents escape hatch", fix)
	}
	if failed := stepEvents(t, cmd.Stdout, "failed"); !contains(failed, "sync-agents") {
		t.Errorf("failed steps = %v, want sync-agents", failed)
	}
	if args, ran := h.nhArgs(); ran {
		t.Errorf("nh ran despite a failed agent sync: %v", args)
	}
}

// --skip-agents is the documented way out of the gate above. It has to
// converge the Nix plane without touching the agent plane at all.
func TestApplySkipAgentsBypassesTheGate(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withNhStub().
		withStateFile(buildStateTOML(stateOverrides{})).
		withAgentsManifest(agentsManifest("0.8.2", "solaris-sparc")).
		withAgentStub("herdr", "herdr 0.1.0")

	cmd := h.run("apply", "--json", "--yes", "--skip-agents")
	if cmd.ExitCode != 0 {
		t.Fatalf("apply --skip-agents exit = %d; stdout: %s\nstderr: %s", cmd.ExitCode, cmd.Stdout, cmd.Stderr)
	}
	if _, ran := h.nhArgs(); !ran {
		t.Error("nh stub never ran; --skip-agents must still activate")
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
