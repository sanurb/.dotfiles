package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
)

// TestPlanIgnoresManagedDirectoryProjections ensures an existing Home Manager
// directory symlink is not mistaken for real files that need quarantining.
func TestPlanIgnoresManagedDirectoryProjections(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withStateFile(buildStateTOML(stateOverrides{}))

	projections := []struct {
		relHome      string
		relWorkspace string
		seedFile     string
	}{
		{relHome: ".config/ghostty", relWorkspace: "config/ghostty", seedFile: "config"},
		{relHome: ".config/nvim", relWorkspace: "config/nvim", seedFile: "init.lua"},
	}
	for _, projection := range projections {
		source := filepath.Join(h.Workspace, projection.relWorkspace)
		mustWrite(t, filepath.Join(source, projection.seedFile), "managed by the workspace\n")
		destination := filepath.Join(h.Home, projection.relHome)
		mustMkdir(t, filepath.Dir(destination))
		if err := os.Symlink(source, destination); err != nil {
			t.Fatalf("create managed directory projection %s: %v", projection.relHome, err)
		}
	}

	got := h.run("plan", "--json")
	assertEqual(t, got.ExitCode, 0)

	envelope := decodeSuccess(t, got.Stdout)
	result := mustObject(t, envelope, "result")
	for _, rawStep := range mustArray(t, result, "steps") {
		step, ok := rawStep.(map[string]any)
		if !ok {
			t.Fatalf("plan step has unexpected type %T", rawStep)
		}
		if step["kind"] == plan.KindSnapshotConflicts {
			t.Fatalf("managed directory projections must not be snapshot conflicts: %v", step)
		}
	}
}
