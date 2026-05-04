package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/applied"
	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/nix"
	"github.com/sanurb/.dotfiles/apps/cli/internal/rollback"
)

const cmdRollbackSummary = "Switch Home Manager to a prior generation"

// runRollback implements `dots rollback [generation]`. With no
// argument it asks the underlying tool (nh or home-manager) for its
// notion of "previous"; with a generation number it switches to that
// specific one.
//
// We prefer `nh home rollback` when present and fall back to
// `home-manager` directly otherwise. Both are subprocesses with stdio
// inherited so output reads natively.
func runRollback(rest []string) int {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var common cliflags.Common
	common.Bind(fs)

	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "rollback: too many arguments; usage: dots rollback [generation]")
		return exitcode.Misuse
	}

	var generation string
	if fs.NArg() == 1 {
		gen := fs.Arg(0)
		if _, err := strconv.Atoi(gen); err != nil {
			fmt.Fprintf(os.Stderr, "rollback: %q is not a generation number\n", gen)
			fmt.Fprintln(os.Stderr, "  what: positional must be an integer or absent")
			fmt.Fprintln(os.Stderr, "  next: try `dots rollback` (previous) or `dots rollback 42`")
			return exitcode.Misuse
		}
		generation = gen
	}

	args, err := rollback.Resolve(generation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rollback: no rollback tool available:")
		fmt.Fprintln(os.Stderr, "  what: neither `nh` nor `home-manager` is on PATH")
		fmt.Fprintln(os.Stderr, "  why:  Home Manager activation has not bootstrapped this host")
		fmt.Fprintln(os.Stderr, "  next: run `dots doctor` to diagnose, then `dots apply`")
		return exitcode.PreFlight
	}

	if common.DryRun {
		fmt.Fprintf(os.Stderr, "would run: %s\n", strings.Join(args, " "))
		return exitcode.Success
	}

	runErr := nix.Cmd{
		Name:   args[0],
		Args:   args[1:],
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}.Run(context.Background())
	if runErr != nil {
		if code, ok := nix.IsExit(runErr); ok {
			fmt.Fprintf(os.Stderr, "rollback: `%s` exited %d\n", strings.Join(args, " "), code)
			return code
		}
		fmt.Fprintln(os.Stderr, "rollback: subprocess failed:")
		fmt.Fprintf(os.Stderr, "  what: could not execute `%s`\n", strings.Join(args, " "))
		fmt.Fprintf(os.Stderr, "  why:  %v\n", runErr)
		fmt.Fprintln(os.Stderr, "  next: ensure the tool is installed, or run `dots doctor`")
		return exitcode.Failure
	}

	// Update the applied receipt: we don't know the prior plan's hash
	// (rollback is by-generation, not by-plan), so clear it. Profile
	// stays at whatever the workspace state file declares — that's
	// still the user's intent.
	if path, err := applied.DefaultPath(); err == nil {
		prior, _, _ := applied.Load(path)
		_ = applied.Save(path, applied.State{
			SchemaVersion: applied.SchemaVersion,
			PlanHash:      "",
			Profile:       prior.Profile,
			AppliedAt:     time.Now().UTC(),
		})
	}

	fmt.Fprintln(os.Stderr, "✓ rollback complete")
	return exitcode.Success
}
