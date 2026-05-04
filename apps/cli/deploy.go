package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/sanurb/.dotfiles/apps/cli/internal/bootstrap"
	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// syncMoonHooksSilent reconciles .git/hooks/{pre-commit,pre-push}
// with the declarative `vcs.hooks` block in .moon/workspace.yml.
// It is the enforcement edge of "Declare, Don't Script": a fresh
// clone, a hand-edited hook, or a stale hook from before the Moon
// migration all converge to the workspace spec on the next sync.
//
// Output is discarded — `moon sync hooks` is idempotent and chatty;
// the wizard owns the user-visible surface. Failure is swallowed for
// the same reason the prior treefmt pass was: a missing `moon` (e.g.
// pre-bootstrap) or a transient permission error on .git/hooks must
// not block a sync. The doctor's formatting check still gates deploy,
// so real drift surfaces there.
func syncMoonHooksSilent() {
	if _, err := exec.LookPath("moon"); err != nil {
		return
	}
	cmd := exec.Command("moon", "sync", "hooks")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

// runDeploy realizes the workspace via `moon run modules:deploy`,
// auto-bootstrapping prerequisites with explicit consent first. ADR-0010
// records the contract.
//
// Bootstrap order is Nix → workspace because the dev shell (and
// therefore moon) requires Nix, but the workspace can be cloned by
// any user with git on $PATH.
//
// Exit codes: 0 success; 2 user declined a prereq; 1 internal failure.
func runDeploy() int {
	if !bootstrap.NixPresent() {
		consented, err := bootstrap.OfferNixInstall(os.Stdout, os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "deploy:", err)
			return 1
		}
		if !consented {
			return 2
		}
		// The just-installed nix is not reachable in this process —
		// the installer mutates shell rc files for future shells, not
		// the current PATH. Tell the user and stop.
		fmt.Println()
		fmt.Println("Nix installed. Open a new shell and re-run `dots deploy` to continue.")
		return 0
	}

	if _, err := workspace.Root(); err != nil {
		path, err := bootstrap.OfferWorkspaceClone(os.Stdout, os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "deploy:", err)
			return 1
		}
		if path == "" {
			return 2
		}
		// The cloned tree is reachable in this process — chdir and
		// continue to moon, which finds .moon/workspace.yml from cwd.
		if err := os.Chdir(path); err != nil {
			fmt.Fprintln(os.Stderr, "deploy: chdir to clone:", err)
			return 1
		}
	}

	return run("moon", "run", ui.MoonDeployTask)
}

func run(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, name, "failed:", err)
		return 1
	}
	return 0
}
