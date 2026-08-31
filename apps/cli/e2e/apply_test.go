package e2e

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
)

func TestApply(t *testing.T) {
	t.Run("when user runs apply --yes from a bare shell, then nh receives the right argv for this host", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withNhStub().
			withStateFile(buildStateTOML(stateOverrides{}))

		h.run("apply", "--yes")

		args, ok := h.nhArgs()
		want := []string{
			"home", "switch",
			"--show-activation-logs",
			"--backup-extension", "backup",
			"-c", expectedNixSystem(), ".",
			"--", "--impure", "--accept-flake-config",
		}
		assertEqual(t, ok, true)
		assertEqual(t, args, want)
	})

	t.Run("when Home Manager consumes a proto runtime, then runtimes install before activation", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withStub("proto", `#!/bin/sh
if [ "$1" = "use" ]; then
  : > "$NH_RECORD.runtimes-ready"
fi
exit 0
`).
			withStateFile(buildStateTOML(stateOverrides{}))
		h.installStub(filepath.Join(h.Workspace, ".devenv", "profile", "bin", "nh"), `#!/bin/sh
case "$1" in
  --version) echo "nh 4.3.2-stub"; exit 0 ;;
esac
if [ ! -f "$NH_RECORD.runtimes-ready" ]; then
  echo "nh ran before proto use" >&2
  exit 91
fi
printf "%s\n" "$@" > "$NH_RECORD"
exit 0
`)

		got := h.run("apply", "--yes")

		assertEqual(t, got.ExitCode, 0)
		_, activated := h.nhArgs()
		assertEqual(t, activated, true)
	})

	t.Run("when user runs apply --dry-run, then plan is rendered to stdout and nh is never invoked", func(t *testing.T) {
		h := newHarness(t).
			withStub("nix", nixStubBody).
			withNhStub().
			withStateFile(buildStateTOML(stateOverrides{Shell: "zsh"}))

		got := h.run("apply", "--dry-run")

		_, invoked := h.nhArgs()
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
// equality with full diff on failure.
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
