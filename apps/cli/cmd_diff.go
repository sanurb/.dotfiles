package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
)

const cmdDiffSummary = "Show what `dots apply` would change (sugar over `dots plan`)"

// runDiff implements `dots diff [profile]`. It shares the producer
// (computePlan) with plan and apply, but always renders the
// human-readable view — `--json` is intentionally NOT honored, because
// the value of `diff` is the visual quick-look. Users who want JSON
// already have `dots plan --json`.
func runDiff(rest []string) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var common cliflags.Common
	common.Bind(fs)

	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	profile := common.Profile
	if profile == "" && fs.NArg() > 0 {
		profile = fs.Arg(0)
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "diff: too many arguments; usage: dots diff [profile]")
		return exitcode.Misuse
	}

	p, err := computePlan(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "diff: compute failed:")
		fmt.Fprintln(os.Stderr, "  what: cannot inspect host")
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: rerun with -v to see more, or check $HOME readability")
		return exitcode.Failure
	}

	renderPlan(os.Stdout, p, !common.NoColor)

	if len(p.Steps) == 0 {
		// NoOp signals "system is already converged"; useful for CI
		// gates that want to short-circuit on a clean tree.
		return exitcode.NoOp
	}
	return exitcode.Success
}
