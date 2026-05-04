// Package preflight is the gate `dots apply` runs before invoking
// `nh home switch`. Deliberately narrower than `dots doctor`: doctor
// validates the full dev environment (LSPs, formatters, dev-shell
// tooling); preflight checks only what activation actually needs.
//
// The narrower scope is load-bearing: the full doctor flags missing
// gopls/treefmt/etc. as PreFlight failures, but those tools have no
// bearing on whether `nh home switch` will succeed. Surfacing them
// here would gate apply on dev-shell health, which is the wrong
// trade-off for a verb the user runs to converge a host.
//
// Apply needs three things, and that is the entire surface:
//   - the workspace is reachable (the flake lives there);
//   - nh resolves through the activation env's augmented PATH;
//   - nix resolves on PATH (nh shells out to it).
//
// The state file is intentionally not checked: plan computation
// already handled missing state with a default profile.
package preflight

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sanurb/.dotfiles/apps/cli/internal/activation"
	"github.com/sanurb/.dotfiles/apps/cli/internal/nix"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// User-facing failure messages. Constants so the wording is one piece
// of evidence under test, not three string-literal echoes.
const (
	msgNh  = "nh: not on PATH and not at <ws>/.devenv/profile/bin/nh"
	msgNix = "nix: not on PATH"
)

// Result is the verdict of a preflight pass. A zero value (no
// failures) is the success state — Result{}.OK() reports true.
type Result struct {
	Failures []string
}

// OK reports whether every probe passed.
func (r Result) OK() bool { return len(r.Failures) == 0 }

// Probes captures the outcome of each preflight probe. nil means
// the probe passed. Used as the input shape for From — the unit-test
// seam — so callers don't have to remember which positional error
// goes where.
type Probes struct {
	Workspace error
	Nh        error
	Nix       error
}

// Check probes the host and assembles a Result. Impure — reads
// os.Environ, the workspace cache, and PATH. Use From for unit tests
// that need to assert Result wording without monkey-patching the
// host.
func Check() Result {
	root, wsErr := workspace.Root()
	env := activation.Build(os.Environ(), root)
	_, nhErr := activation.LookPathIn(nix.ToolNh, env)
	_, nixErr := exec.LookPath("nix")
	return From(Probes{Workspace: wsErr, Nh: nhErr, Nix: nixErr})
}

// From assembles a Result from probe outcomes. Pure: same inputs,
// same Result, regardless of host.
func From(p Probes) Result {
	var fs []string
	if p.Workspace != nil {
		fs = append(fs, fmt.Sprintf("workspace: %v", p.Workspace))
	}
	if p.Nh != nil {
		fs = append(fs, msgNh)
	}
	if p.Nix != nil {
		fs = append(fs, msgNix)
	}
	return Result{Failures: fs}
}
