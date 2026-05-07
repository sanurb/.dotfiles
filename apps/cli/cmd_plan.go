package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/envelope"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
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
		if common.JSON {
			_ = envelope.Fail(os.Stdout, commandLine("plan", rest),
				envelope.Wrap(envelope.CodeInternalError, err).
					WithFix("Re-run with -v to see more, or check $HOME readability."))
			return exitcode.Failure
		}
		fmt.Fprintln(os.Stderr, "plan: compute failed:")
		fmt.Fprintln(os.Stderr, "  what: cannot inspect host")
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: rerun with -v to see more, or check $HOME readability")
		return exitcode.Failure
	}

	// --out FILE always writes the raw Plan as a replay artifact —
	// `dots apply --plan FILE` consumes that exact format. The
	// envelope on stdout (under --json) is a separate consumer; we
	// don't duplicate the full plan into both, only the summary.
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			if common.JSON {
				_ = envelope.Fail(os.Stdout, commandLine("plan", rest),
					envelope.Wrap(envelope.CodeInvalidArgument, fmt.Errorf("open %s: %w", outPath, err)))
				return exitcode.Failure
			}
			fmt.Fprintln(os.Stderr, "plan: open:", err)
			return exitcode.Failure
		}
		if encErr := p.Encode(f); encErr != nil {
			_ = f.Close()
			if common.JSON {
				_ = envelope.Fail(os.Stdout, commandLine("plan", rest),
					envelope.Wrap(envelope.CodeInternalError, encErr))
				return exitcode.Failure
			}
			fmt.Fprintln(os.Stderr, "plan: encode:", encErr)
			return exitcode.Failure
		}
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintln(os.Stderr, "plan: close:", cerr)
		}
		if common.JSON {
			_ = envelope.OK(os.Stdout, commandLine("plan", rest),
				planSummaryJSON(p, outPath),
				planActionsWithFile(outPath))
			return exitcode.Success
		}
		// Non-JSON --out path stays as today: file holds the plan,
		// stdout stays empty (no human render to a file).
		return exitcode.Success
	}

	if common.JSON {
		_ = envelope.OK(os.Stdout, commandLine("plan", rest), p, planActions())
		return exitcode.Success
	}

	renderPlan(os.Stdout, p, !common.NoColor)
	fmt.Println()
	fmt.Println("Run `dots apply` to execute, or `dots plan --out FILE` to save.")
	return exitcode.Success
}

// planSummaryJSON is the result body when --out wrote the plan to a
// file: a small summary that points to the full artifact rather than
// duplicating it on stdout. The summary fields are stable; the full
// plan is read from out_path by `dots apply --plan FILE`.
type planSummaryBody struct {
	OutPath     string `json:"out_path"`
	Hash        string `json:"hash"`
	Profile     string `json:"profile"`
	GeneratedAt string `json:"generated_at"`
	StepCount   int    `json:"step_count"`
}

func planSummaryJSON(p plan.Plan, outPath string) planSummaryBody {
	return planSummaryBody{
		OutPath:     outPath,
		Hash:        p.Hash,
		Profile:     p.Profile,
		GeneratedAt: p.GeneratedAt.UTC().Format(time.RFC3339),
		StepCount:   len(p.Steps),
	}
}

func planActions() []envelope.Action {
	return []envelope.Action{
		{Command: "dots apply", Description: "Execute this plan."},
		{
			Command:     "dots plan --out <path>",
			Description: "Save the plan to a file for replay or review.",
			Params: map[string]envelope.ActionParam{
				"path": {Description: "Path to write the plan JSON.", Required: true},
			},
		},
	}
}

func planActionsWithFile(outPath string) []envelope.Action {
	return []envelope.Action{
		{
			Command:     "dots apply --plan <path>",
			Description: "Replay the saved plan, refusing to run if a fresh compute would diverge.",
			Params: map[string]envelope.ActionParam{
				"path": {Description: "Plan file written by `dots plan --out`.", Value: outPath, Required: true},
			},
		},
		{Command: "dots apply", Description: "Recompute and execute the plan in one step."},
	}
}
