package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/agents"
	"github.com/sanurb/.dotfiles/apps/cli/internal/envelope"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// tempWorkspace points workspace.Root() at a throwaway tree holding the
// given manifest, and restores the process-wide cache afterwards.
// MOON_WORKSPACE_ROOT is the documented short-circuit in
// internal/workspace, so no marker files are needed.
func tempWorkspace(t *testing.T, manifest string) string {
	t.Helper()
	root := t.TempDir()
	if manifest != "" {
		path := filepath.Join(root, agents.RelPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}
	t.Setenv("MOON_WORKSPACE_ROOT", root)
	workspace.Reset()
	t.Cleanup(workspace.Reset)
	return root
}

// A row whose probe names a binary that cannot exist always reads as
// missing, so these tests exercise the converge path without depending
// on what the developer's host happens to have installed.
const absentProbe = "dots-agent-that-cannot-exist --version"

// A converged plane must not fail the apply, and must not be silent
// about having run.
func TestSyncAgentsStepNoOpsOnNativeOnlyManifest(t *testing.T) {
	tempWorkspace(t, `
schema_version = 1
[agents.claude-code]
provider = "native"
probe    = "`+absentProbe+`"
`)
	var log bytes.Buffer
	if problem := syncAgentsStep(context.Background(), nil, &log); problem != nil {
		t.Fatalf("syncAgentsStep = %v, want nil", problem)
	}
	if !strings.Contains(log.String(), "already converged") {
		t.Errorf("log = %q, want an already-converged note", log.String())
	}
}

// A malformed manifest is a schema mismatch: it must stop the apply
// before activation, with the code the `dots agents` verb uses for the
// same condition, so both surfaces agree.
func TestSyncAgentsStepRejectsMalformedManifest(t *testing.T) {
	tempWorkspace(t, "schema_version = 1\n[agents.x]\nverison = \"1.0.0\"\n")
	var log bytes.Buffer
	problem := syncAgentsStep(context.Background(), nil, &log)
	if problem == nil {
		t.Fatal("syncAgentsStep accepted a malformed manifest")
	}
	if problem.Code != envelope.CodeAgentManifestInvalid {
		t.Errorf("code = %q, want %q", problem.Code, envelope.CodeAgentManifestInvalid)
	}
	if got := mapCodeToExit(problem.Code); got != 4 {
		t.Errorf("exit code = %d, want 4 (PreFlight)", got)
	}
}

// A genuine convergence failure must stop the apply rather than let
// activation run against a half-converged plane — herdr's activation
// hook installs plugins through a binary this step is responsible for.
// The digest map here names no platform dots can run on, so the failure
// happens before any network I/O and the test stays hermetic.
func TestSyncAgentsStepFailsOnUninstallableAgent(t *testing.T) {
	tempWorkspace(t, `
schema_version = 1
[agents.herdr]
provider = "github-release"
version  = "0.8.2"
repo     = "herdrdev/herdr"
asset    = "herdr-{os}-{arch}"
probe    = "`+absentProbe+`"
[agents.herdr.sha256]
solaris-sparc = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
`)
	var log bytes.Buffer
	problem := syncAgentsStep(context.Background(), nil, &log)
	if problem == nil {
		t.Fatal("syncAgentsStep succeeded against an uninstallable agent")
	}
	if problem.Code != envelope.CodeAgentSyncFailed {
		t.Errorf("code = %q, want %q", problem.Code, envelope.CodeAgentSyncFailed)
	}
	// The remediation has to name the escape hatch, or a user blocked
	// here has no way to converge the Nix plane.
	if !strings.Contains(problem.EffectiveFix(), "--skip-agents") {
		t.Errorf("fix = %q, want it to mention --skip-agents", problem.EffectiveFix())
	}
}

// The bun provider needs a bun that `proto use` may only have installed
// moments earlier in the same apply. That ordering artifact must not
// fail the run — it is the same policy install-runtimes applies to a
// proto that is not on PATH yet.
func TestSyncAgentsStepSkipsWhenProviderToolchainMissing(t *testing.T) {
	tempWorkspace(t, `
schema_version = 1
[agents.pi]
provider = "bun"
version  = "0.85.1"
package  = "@earendil-works/pi-coding-agent"
probe    = "`+absentProbe+`"
`)
	var log bytes.Buffer
	// An empty activation env resolves nothing, so bun reads as absent.
	if problem := syncAgentsStep(context.Background(), []string{"PATH="}, &log); problem != nil {
		t.Fatalf("syncAgentsStep = %v, want nil (skip, not failure)", problem)
	}
	if !strings.Contains(log.String(), "skipped") {
		t.Errorf("log = %q, want a skip note", log.String())
	}
}

// No manifest at all is not an error: a host that has not adopted the
// plane still applies cleanly.
func TestSyncAgentsStepToleratesAbsentManifest(t *testing.T) {
	tempWorkspace(t, "")
	var log bytes.Buffer
	if problem := syncAgentsStep(context.Background(), nil, &log); problem != nil {
		t.Fatalf("syncAgentsStep = %v, want nil", problem)
	}
}
