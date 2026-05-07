package main

import (
	"fmt"
	"os"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/envelope"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
)

// runHeadlessInstall is the non-interactive parallel of the wizard.
//
// Contract:
//   - state already loaded into deps.Initial (canonical workspace file
//     or a --config override; the load itself happens in newWizardDeps).
//   - Validate, then atomically persist to the canonical location so
//     home.nix sees the resolved persona on the next read.
//   - --yes hands off to `dots apply --yes --non-interactive` so the
//     scan/snapshot/realize triad runs in its native verb. Without
//     --yes we stop after persist and print the same next-step hint
//     the wizard prints when realization is declined.
//
// The scan/snapshot phase is intentionally NOT replicated here: that
// logic lives in `dots apply` and is driven by the same flags. Forking
// it into init would create two sources of truth for "headless
// realization" and is the exact tech-debt shape this refactor exists
// to avoid.
func runHeadlessInstall(deps ui.Deps, common cliflags.Common) int {
	command := "dots init --non-interactive"
	if common.Yes {
		command += " --yes"
	}

	plan, err := planHeadlessInstall(deps.Initial, common)
	if err != nil {
		if common.JSON {
			_ = envelope.Fail(os.Stdout, command,
				envelope.Wrap(envelope.CodeStateInvalid, err))
			return exitcode.Misuse
		}
		fmt.Fprintln(os.Stderr, "init:", err)
		return exitcode.Misuse
	}

	if err := deps.StatePersister.SaveState(deps.Initial); err != nil {
		if common.JSON {
			_ = envelope.Fail(os.Stdout, command,
				envelope.Wrap(envelope.CodeInternalError, fmt.Errorf("persist state: %w", err)))
			return exitcode.Failure
		}
		fmt.Fprintln(os.Stderr, "init: persist state:", err)
		return exitcode.Failure
	}

	if !plan.realize {
		if common.JSON {
			_ = envelope.OK(os.Stdout, command, headlessInstallResultBodyFromState(deps),
				headlessNextActionsWithoutRealize())
			return exitcode.Success
		}
		printInstallNextSteps()
		return exitcode.Success
	}

	self, rerr := resolveSelf()
	if rerr != nil {
		if common.JSON {
			_ = envelope.Fail(os.Stdout, command,
				envelope.Wrap(envelope.CodeInternalError, rerr))
			return exitcode.Failure
		}
		fmt.Fprintln(os.Stderr, "apply:", rerr)
		return exitcode.Failure
	}
	// Hand-off to apply: the child emits its own envelope (snapshot
	// for --dry-run, NDJSON stream + terminal for the real apply).
	// init's exit code mirrors the child's; init itself emits nothing
	// further so stdout carries exactly one envelope per invocation.
	return run(self, plan.applyArgs...)
}

// headlessInstallResultBody is the result body for the
// no-realize-yet success envelope. It echoes back the persisted
// persona so an agent doesn't have to follow up with `dots profile
// show` to confirm what landed on disk.
type headlessInstallResultBody struct {
	Persisted bool   `json:"persisted"`
	Shell     string `json:"shell"`
	Terminal  string `json:"terminal"`
}

func headlessInstallResultBodyFromState(deps ui.Deps) headlessInstallResultBody {
	return headlessInstallResultBody{
		Persisted: true,
		Shell:     deps.Initial.Pillars.Shell,
		Terminal:  deps.Initial.Pillars.Terminal,
	}
}

// headlessNextActionsWithoutRealize is the next_actions for the
// "profile saved, didn't realize" path: the natural follow-up is
// `dots apply`, so we put it first with no params (literal command).
func headlessNextActionsWithoutRealize() []envelope.Action {
	return []envelope.Action{
		{Command: "dots apply", Description: "Realize the just-persisted profile."},
	}
}

// headlessPlan is the testable decision the headless install boils
// down to: should we hand off to apply, and if so with which arg list?
// Keeping it pure means a table-driven test exercises every flag
// permutation without spinning up workspaces or processes.
type headlessPlan struct {
	realize   bool
	applyArgs []string
}

// planHeadlessInstall is the pure decision function. Returns an error
// only when the resolved state is invalid — every other branch is
// expressible as (realize, applyArgs).
func planHeadlessInstall(s stateValidator, common cliflags.Common) (headlessPlan, error) {
	if err := s.Validate(); err != nil {
		return headlessPlan{}, err
	}
	if !common.Yes {
		return headlessPlan{realize: false}, nil
	}
	return headlessPlan{realize: true, applyArgs: applyArgsFromCommon(common)}, nil
}

// stateValidator narrows state.State to the single method this package
// exercises. Defined here (rather than imported from state) so the
// test file can substitute a stub without dragging in the whole state
// package surface — keeps the test boundary thin.
type stateValidator interface {
	Validate() error
}

// applyArgsFromCommon builds the argv for the `dots apply` subprocess.
// --yes and --non-interactive are unconditional: this code path is
// only reached when both are in effect for init, and re-asserting them
// makes the child invocation legible without consulting environment
// state. Other flags are forwarded verbatim so behavior in the child
// matches behavior the user requested for the parent.
func applyArgsFromCommon(common cliflags.Common) []string {
	args := []string{"apply", "--yes", "--non-interactive"}
	if common.JSON {
		args = append(args, "--json")
	}
	if common.NoColor {
		args = append(args, "--no-color")
	}
	if common.Quiet {
		args = append(args, "--quiet")
	}
	for i := 0; i < common.Verbose; i++ {
		args = append(args, "-v")
	}
	if common.Profile != "" {
		args = append(args, "--profile", common.Profile)
	}
	if common.DryRun {
		args = append(args, "--dry-run")
	}
	return args
}
