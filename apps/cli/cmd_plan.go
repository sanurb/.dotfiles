package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
)

const cmdPlanSummary = "Compute the plan that `dots apply` would execute"

// runPlan implements `dots plan [profile] [--json] [--out FILE]`.
// The plan is computed via computePlan — the same producer `dots apply`
// uses — so what `plan` shows is exactly what `apply` will run.
func runPlan(rest []string) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var common cliflags.Common
	common.Bind(fs)

	var outPath string
	fs.StringVar(&outPath, "out", "", "write plan to FILE instead of stdout")

	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	profile := common.Profile
	if profile == "" && fs.NArg() > 0 {
		profile = fs.Arg(0)
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "plan: too many arguments; usage: dots plan [profile]")
		return exitcode.Misuse
	}

	p, err := computePlan(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plan: compute failed:")
		fmt.Fprintln(os.Stderr, "  what: cannot inspect host")
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: rerun with -v to see more, or check $HOME readability")
		return exitcode.Failure
	}

	w, closer, err := openPlanOutput(outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plan:", err)
		return exitcode.Failure
	}
	defer closer()

	if common.JSON {
		if err := p.Encode(w); err != nil {
			fmt.Fprintln(os.Stderr, "plan: encode:", err)
			return exitcode.Failure
		}
		return exitcode.Success
	}

	renderPlan(w, p, !common.NoColor)
	if outPath == "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Run `dots apply` to execute, or `dots plan --out FILE` to save.")
	}
	return exitcode.Success
}

// openPlanOutput resolves --out: empty means stdout (no-op closer);
// otherwise create the file and return a closer that propagates close
// errors via stderr at defer time.
func openPlanOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintln(os.Stderr, "plan: close:", cerr)
		}
	}, nil
}
