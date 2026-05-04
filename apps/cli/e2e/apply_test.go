package e2e

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
)

func TestApply(t *testing.T) {
	t.Run("when user runs apply --yes from a bare shell, then moon receives PROTO_HOME pointing at the workspace .proto", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withMoonStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		h.run("apply", "--yes")

		env, _ := h.moonEnv()
		assertEqual(t, env["PROTO_HOME"], filepath.Join(h.Workspace, ".proto"))
	})

	t.Run("when user runs apply --yes from a bare shell, then moon's PATH leads with proto shims and proto bin from the workspace", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withMoonStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		h.run("apply", "--yes")

		env, _ := h.moonEnv()
		want := []string{
			filepath.Join(h.Workspace, ".proto", "shims"),
			filepath.Join(h.Workspace, ".proto", "bin"),
		}
		assertPathLeading(t, env["PATH"], want)
	})

	t.Run("when user runs apply --yes from a bare shell, then moon receives DOTS_NIX_SYSTEM matching the host architecture", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withMoonStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		h.run("apply", "--yes")

		env, _ := h.moonEnv()
		assertEqual(t, env["DOTS_NIX_SYSTEM"], expectedNixSystem())
	})

	t.Run("when user runs apply --dry-run, then plan is rendered to stdout and moon is never invoked", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withMoonStub().
			withStateFile(buildStateTOML(stateOverrides{Shell: "zsh"}))

		got := h.run("apply", "--dry-run")

		_, invoked := h.moonEnv()
		assertEqual(t, got.ExitCode, 0)
		assertContains(t, got.Stdout, "apply profile zsh")
		assertEqual(t, invoked, false)
	})
}

// expectedNixSystem reuses the production formula by calling into
// plan.Host.NixIdent — when the formula changes for any reason the
// e2e expectation tracks it automatically.
func expectedNixSystem() string {
	return plan.CurrentHost().NixIdent()
}

// assertEqual is the package's single comparison primitive: deep
// equality with full diff on failure. Tests stay flat by routing
// every assertion through it.
func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\n got:  %#v\nwant: %#v", got, want)
	}
}

// assertContains is for partial-match assertions on multi-line text
// surfaces (stdout / stderr) where we want to bind on a phrase, not
// the whole envelope.
func assertContains(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Errorf("\n %q\nshould contain\n %q", body, want)
	}
}

// assertPathLeading checks PATH starts with expected entries in order.
// Single failure surface so a wrong order fails as one diff, not
// per-entry.
func assertPathLeading(t *testing.T, path string, want []string) {
	t.Helper()
	got := strings.Split(path, ":")
	if len(got) >= len(want) {
		got = got[:len(want)]
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\n got:  %v\nwant: %v", got, want)
	}
}
