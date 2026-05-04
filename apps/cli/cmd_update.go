package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

const cmdUpdateSummary = "Pull the workspace and re-apply"

// runUpdate implements `dots update [--yes] [--non-interactive]
// [--dry-run]`. It is the standard "stay current" loop: fast-forward
// the workspace, then delegate to runApply so the same converge path
// is exercised regardless of entry point.
func runUpdate(rest []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var common cliflags.Common
	common.Bind(fs)

	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	root, err := workspace.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, "update: workspace required:")
		fmt.Fprintln(os.Stderr, "  what: cannot locate dotfiles workspace")
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: run `dots install` from outside a workspace, or cd into one")
		return exitcode.PreFlight
	}

	pullArgs := []string{"-C", root, "pull", "--ff-only"}

	if common.DryRun {
		fmt.Fprintf(os.Stderr, "would run: git %s\n", strings.Join(pullArgs, " "))
		p, err := computePlan(common.Profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "update: plan compute failed:", err)
			return exitcode.Failure
		}
		renderPlan(os.Stdout, p, !common.NoColor)
		return exitcode.Success
	}

	pull := exec.Command("git", pullArgs...)
	pull.Stdin = os.Stdin
	pull.Stdout = os.Stdout
	pull.Stderr = os.Stderr
	if err := pull.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "update: `git pull --ff-only` exited %d\n", ee.ExitCode())
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "update: git pull failed:")
		fmt.Fprintln(os.Stderr, "  what: could not fast-forward the workspace")
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: resolve the diverging history manually, then re-run `dots update`")
		return exitcode.Failure
	}

	// Forward the relevant flags to runApply. We rebuild the arg slice
	// rather than call runApply with our parsed common because runApply
	// owns its own FlagSet — keeping the CLI surface as the single
	// integration point is simpler than threading a parsed bag.
	applyArgs := forwardApplyFlags(common)
	return runApply(applyArgs)
}

// forwardApplyFlags rebuilds the subset of cliflags.Common values we
// want runApply to honor. Profile is forwarded so `dots update <p>`
// (when we later add a positional) wouldn't lose it; for v1 the
// positional isn't accepted but the plumbing is harmless.
func forwardApplyFlags(c cliflags.Common) []string {
	var args []string
	if c.Yes {
		args = append(args, "--yes")
	}
	if c.NonInteractive {
		args = append(args, "--non-interactive")
	}
	if c.JSON {
		args = append(args, "--json")
	}
	if c.NoColor {
		args = append(args, "--no-color")
	}
	if c.Quiet {
		args = append(args, "--quiet")
	}
	if c.Profile != "" {
		args = append(args, "--profile", c.Profile)
	}
	return args
}
