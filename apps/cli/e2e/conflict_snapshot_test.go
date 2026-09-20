package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
)

// TestPlanIgnoresManagedConfigProjections ensures Home Manager directory
// symlinks and activation-managed child symlinks are not mistaken for real
// files that need quarantining.
func TestPlanIgnoresManagedConfigProjections(t *testing.T) {
	h := newHarness(t).
		withStub("nix", nixStubBody).
		withStateFile(buildStateTOML(stateOverrides{}))

	// Neovim remains a Home Manager directory projection.
	nvimSource := filepath.Join(h.Workspace, "config/nvim")
	mustWrite(t, filepath.Join(nvimSource, "init.lua"), "managed by the workspace\n")
	nvimDestination := filepath.Join(h.Home, ".config/nvim")
	mustMkdir(t, filepath.Dir(nvimDestination))
	if err := os.Symlink(nvimSource, nvimDestination); err != nil {
		t.Fatalf("create managed Neovim projection: %v", err)
	}

	// Ghostty's parent is a real directory because its startup-critical child
	// links deliberately bypass /nix/store.
	ghosttyThemesSource := filepath.Join(h.Workspace, "config/ghostty/themes")
	mustWrite(t, filepath.Join(ghosttyThemesSource, "gentleman"), "managed by the workspace\n")
	ghosttyThemesDestination := filepath.Join(h.Home, ".config/ghostty/themes")
	mustMkdir(t, filepath.Dir(ghosttyThemesDestination))
	if err := os.Symlink(ghosttyThemesSource, ghosttyThemesDestination); err != nil {
		t.Fatalf("create managed Ghostty themes projection: %v", err)
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
			t.Fatalf("managed config projections must not be snapshot conflicts: %v", step)
		}
	}
}
