package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/agents"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
)

// indexOfKind returns the position of the first step with kind, or -1.
func indexOfKind(p plan.Plan, kind string) int {
	for i, s := range p.Steps {
		if s.Kind == kind {
			return i
		}
	}
	return -1
}

// The sync-agents step sits between install-runtimes and apply-profile,
// and both edges are correctness constraints rather than taste:
//
//	after install-runtimes — the bun provider needs the proto-pinned bun
//	before apply-profile   — herdr's activation hook installs plugins
//	                         through the herdr binary this step provides
//
// Reordering either edge silently breaks a fresh-host bootstrap, which
// is exactly the class of bug that survives review.
func TestComputePlanOrdersSyncAgentsBetweenRuntimesAndProfile(t *testing.T) {
	p, err := computePlan("")
	if err != nil {
		t.Skipf("computePlan unavailable in this environment: %v", err)
	}
	runtimes := indexOfKind(p, plan.KindInstallRuntimes)
	sync := indexOfKind(p, plan.KindSyncAgents)
	profile := indexOfKind(p, plan.KindApplyProfile)

	if runtimes < 0 || sync < 0 {
		t.Skipf("plan has no workspace-scoped steps (runtimes=%d sync=%d)", runtimes, sync)
	}
	if !(runtimes < sync) {
		t.Errorf("install-runtimes at %d must precede sync-agents at %d", runtimes, sync)
	}
	if !(sync < profile) {
		t.Errorf("sync-agents at %d must precede apply-profile at %d", sync, profile)
	}
}

// sync-agents mutates the host outside apply-profile, so its presence
// has to defeat the already-applied no-op short-circuit. Without this,
// `dots apply` on a converged Nix plane would skip agent convergence
// entirely and report success.
func TestSyncAgentsIsASideEffectStep(t *testing.T) {
	p := plan.Plan{Steps: []plan.Step{{Kind: plan.KindSyncAgents}}}
	if !p.HasSideEffectSteps() {
		t.Error("sync-agents must count as a side-effect step")
	}
	if isAlreadyApplied(p) {
		t.Error("a plan containing sync-agents must never short-circuit as already-applied")
	}
}

// A provider whose package manager is not on PATH yet is the one
// convergence outcome that must not fail an apply: it is the ordering
// artifact of the very run that installs that package manager. Every
// other failure is real.
func TestConvergeAgentsSeparatesToolchainSkipsFromFailures(t *testing.T) {
	missingToolchain := agents.Status{
		Agent: agents.Agent{
			Name: "pi", Provider: agents.ProviderBun,
			Version: "1.0.0", Package: "p", Probe: "pi --version",
		},
		State: agents.StateMissing,
	}
	// No digest for any platform this host can be: fails before any
	// network I/O, so the test stays hermetic.
	realFailure := agents.Status{
		Agent: agents.Agent{
			Name: "herdr", Provider: agents.ProviderGitHubRelease,
			Version: "1.0.0", Repo: "o/r", Asset: "herdr-{os}-{arch}",
			Probe:  "herdr --version",
			SHA256: map[string]string{"solaris-sparc": strings.Repeat("a", 64)},
		},
		State: agents.StateMissing,
	}

	neverResolves := agents.Resolver(func(string) (string, error) { return "", exec.ErrNotFound })
	out := convergeAgents(
		context.Background(),
		[]agents.Status{missingToolchain, realFailure},
		neverResolves,
		func(string, ...any) {},
	)

	if len(out.Skipped) != 1 || out.Skipped[0].Agent.Name != "pi" {
		t.Errorf("Skipped = %v, want just pi", names(out.Skipped))
	}
	if len(out.Failed) != 1 || out.Failed[0].Agent.Name != "herdr" {
		t.Errorf("Failed = %v, want just herdr", names(out.Failed))
	}
	if len(out.Synced) != 0 {
		t.Errorf("Synced = %v, want none", names(out.Synced))
	}
}

func names(ss []agents.Status) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Agent.Name
	}
	return out
}
