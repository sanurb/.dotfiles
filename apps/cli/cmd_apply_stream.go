package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/activation"
	"github.com/sanurb/.dotfiles/apps/cli/internal/applied"
	"github.com/sanurb/.dotfiles/apps/cli/internal/envelope"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/loginshell"
	"github.com/sanurb/.dotfiles/apps/cli/internal/nix"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
	"github.com/sanurb/.dotfiles/apps/cli/internal/preflight"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// runApplyStreaming is the --json path of `dots apply`. It mirrors
// the canonical step loop in runApply but emits NDJSON events rather
// than prose, captures subprocess stderr to a per-run log file, and
// terminates with a typed success or error envelope.
//
// Bootstrap steps (KindBootstrapNix, KindCloneWorkspace) are
// excluded: both require interactive stdin consent, which is
// incompatible with NDJSON-on-stdout. If the plan contains either,
// we emit a BOOTSTRAP_REQUIRED error envelope before opening the
// stream — agents resolve the prereq and retry.
func runApplyStreaming(p plan.Plan, env []string, profile string, rest []string, noPreflight bool) int {
	command := commandLine("apply", rest)

	if needsInteractiveConsent(p) {
		_ = envelope.Fail(os.Stdout, command,
			envelope.New(envelope.CodeBootstrapRequired,
				"plan includes bootstrap or clone step which cannot run under --json"))
		return exitcode.Misuse
	}

	runID := envelope.NewULID()
	logFile, logPath, err := envelope.OpenRunLog(runID, "apply.log")
	if err != nil {
		_ = envelope.Fail(os.Stdout, command,
			envelope.Wrap(envelope.CodeInternalError, err).WithRunID(runID))
		return exitcode.Failure
	}
	defer logFile.Close()

	stream := envelope.NewStream(os.Stdout, command, runID, logPath, nil)
	if err := stream.Start(); err != nil {
		// stdout is broken — no useful recovery beyond surfacing the
		// underlying error to stderr and bailing.
		fmt.Fprintln(os.Stderr, "apply: stream start:", err)
		return exitcode.Failure
	}

	// Mirror runApply's parallel install-runtimes optimization. The
	// goroutine writes its stderr to the log file (via the same
	// activation helpers), so step events still bracket each phase
	// even when they overlap.
	var runtimesAsync <-chan int
	if shouldParallelizeRuntimes(p, env) {
		ch := make(chan int, 1)
		runtimesAsync = ch
		go func() { ch <- runInstallRuntimesTo(env, logFile) }()
	}

	for _, step := range p.Steps {
		stepName := step.Kind
		_ = stream.StepStarted(stepName)

		var problem *envelope.Problem
		switch step.Kind {
		case plan.KindSnapshotConflicts:
			if err := snapshotConflicts(step.Effects); err != nil {
				problem = envelope.Wrap(envelope.CodeInternalError, err)
			}

		case plan.KindApplyProfile:
			if _, err := workspace.Root(); err != nil {
				problem = envelope.Wrap(envelope.CodeWorkspaceNotFound, err)
				break
			}
			if !noPreflight {
				if r := preflight.Check(); !r.OK() {
					problem = envelope.New(envelope.CodePreflightFailed,
						"doctor preflight surfaced a SevFail").
						WithFix("Run `dots doctor` to see the failing item; fix it, then re-run.")
					break
				}
			}
			if code := runHomeActivationTo(profile, env, logFile); code != exitcode.Success {
				problem = envelope.New(envelope.CodeActivationFailed,
					fmt.Sprintf("nh home switch exited %d", code))
				break
			}
			// Login-shell hookup is best-effort, same as the prose
			// path: a failure here doesn't fail apply, it logs and
			// continues.
			if _, err := loginshell.Apply(context.Background(), profile, logFile); err != nil {
				fmt.Fprintln(logFile, "login-shell:", err)
			}

		case plan.KindInstallRuntimes:
			var code int
			if runtimesAsync != nil {
				code = <-runtimesAsync
			} else {
				code = runInstallRuntimesTo(env, logFile)
			}
			if code != exitcode.Success {
				problem = envelope.New(envelope.CodeBuildFailed,
					fmt.Sprintf("install-runtimes exited %d", code))
			}

		default:
			problem = envelope.New(envelope.CodeInternalError,
				fmt.Sprintf("unknown step kind %q (plan schema drift?)", step.Kind))
		}

		if problem != nil {
			_ = stream.StepFailed(stepName)
			_ = stream.Failure(problem)
			return mapCodeToExit(problem.Code)
		}
		_ = stream.StepCompleted(stepName)
	}

	// Persist the receipt — same best-effort policy as the prose path.
	if path, err := applied.DefaultPath(); err == nil {
		_ = applied.Save(path, applied.State{
			SchemaVersion: applied.SchemaVersion,
			PlanHash:      p.ConvergedHash(),
			Profile:       p.Profile,
			AppliedAt:     time.Now().UTC(),
		})
	}

	body := applyResultBody{
		StepsExecuted: len(p.Steps),
		PlanHash:      p.ConvergedHash(),
		Profile:       p.Profile,
	}
	_ = stream.Success(body, applyStreamingActions())
	return exitcode.Success
}

// applyResultBody is the result payload of the terminal success
// envelope. Stable structural shape; new fields are additive.
type applyResultBody struct {
	StepsExecuted int    `json:"steps_executed"`
	PlanHash      string `json:"plan_hash"`
	Profile       string `json:"profile"`
}

// applyStreamingActions are the natural follow-ups after a successful
// apply: confirm the realized state, audit the env. `dots status`
// surfaces the new applied receipt; `dots doctor` walks the persona.
func applyStreamingActions() []envelope.Action {
	return []envelope.Action{
		{Command: "dots status", Description: "Confirm the apply landed and the system is converged."},
		{Command: "dots doctor", Description: "Audit the realized environment against the declared persona."},
	}
}

// mapCodeToExit maps an error code back to the existing exit-code
// surface. The exit codes are part of the public CLI contract; the
// envelope's code is the agent's contract. Both must agree on the
// process-level outcome.
func mapCodeToExit(c envelope.Code) int {
	switch c {
	case envelope.CodeWorkspaceNotFound, envelope.CodePreflightFailed, envelope.CodeBootstrapRequired:
		return exitcode.PreFlight
	case envelope.CodeInvalidArgument, envelope.CodeUnknownCommand:
		return exitcode.Misuse
	case envelope.CodeDeclined:
		return exitcode.Declined
	case envelope.CodeAborted:
		return exitcode.Aborted
	default:
		return exitcode.Failure
	}
}

// runHomeActivationTo is the streaming twin of runHomeActivation:
// same nh invocation, same exit-code mapping, but stderr lands in
// the log writer instead of os.Stderr. Stdin is /dev/null since
// streaming apply never has interactive consent to mediate (we
// rejected bootstrap/clone steps before opening the stream).
func runHomeActivationTo(profile string, env []string, logW io.Writer) int {
	sys := plan.CurrentHost().NixIdent()
	if sys == "" {
		fmt.Fprintf(logW, "apply: cannot resolve nix system identifier (GOOS=%s GOARCH=%s)\n", runtime.GOOS, runtime.GOARCH)
		return exitcode.Failure
	}
	nhPath, err := activation.LookPathIn(nix.ToolNh, env)
	if err != nil {
		fmt.Fprintln(logW, "apply: nh not reachable:", err)
		return exitcode.Failure
	}
	runErr := nix.Cmd{
		Name:   nhPath,
		Args:   activationArgs(sys),
		Env:    env,
		Stdin:  nil,
		Stdout: logW,
		Stderr: logW,
	}.Run(context.Background())
	if runErr == nil {
		return exitcode.Success
	}
	if code, ok := nix.IsExit(runErr); ok {
		fmt.Fprintf(logW, "apply: `nh home switch -c %s .` exited %d\n", sys, code)
		return code
	}
	fmt.Fprintln(logW, "apply: nh exec failed:", runErr)
	return exitcode.Failure
}

// runInstallRuntimesTo is the streaming twin of runInstallRuntimes:
// proto invocation with stdout/stderr tee'd to the log file. The
// non-streaming version writes to os.Stderr; both share the same
// process-level outcome semantics. A missing workspace or unreachable
// proto is non-fatal here too — same skip-with-log policy as the
// prose path so a fresh host's first apply still lands.
func runInstallRuntimesTo(env []string, logW io.Writer) int {
	root, err := workspace.Root()
	if err != nil {
		fmt.Fprintln(logW, "install-runtimes: workspace not resolved; skipping")
		return exitcode.Success
	}
	protoPath, err := activation.LookPathIn(nix.ToolProto, env)
	if err != nil {
		fmt.Fprintln(logW, "install-runtimes: proto not yet on PATH; rerun `dots apply` after this activation lands")
		return exitcode.Success
	}
	runErr := nix.Cmd{
		Name:   protoPath,
		Args:   []string{"use"},
		Env:    env,
		Dir:    root,
		Stdin:  nil,
		Stdout: logW,
		Stderr: logW,
	}.Run(context.Background())
	if runErr == nil {
		return exitcode.Success
	}
	if code, ok := nix.IsExit(runErr); ok {
		fmt.Fprintf(logW, "install-runtimes: `proto use` exited %d\n", code)
		return code
	}
	fmt.Fprintln(logW, "install-runtimes: proto exec failed:", runErr)
	return exitcode.Failure
}
