package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/applied"
	"github.com/sanurb/.dotfiles/apps/cli/internal/bootstrap"
	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

const cmdApplySummary = "Apply the plan: bootstrap if needed, snapshot conflicts, realize the profile"

// envPath is the only env key shared between activationEnv and
// prependPathEntries. PROTO_HOME / DOTS_NIX_SYSTEM are gone now that
// the deploy path no longer goes through moon.
const envPath = "PATH"

// runApply implements `dots apply [profile] [--plan FILE] [--dry-run]
// [--yes] [--non-interactive] [--json]`. It is the canonical converge
// verb: the only path through which the system mutates from "what's on
// disk" toward "what the plan says."
func runApply(rest []string) int {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var common cliflags.Common
	common.Bind(fs)

	var planPath string
	fs.StringVar(&planPath, "plan", "", "execute plan from FILE (must match a fresh computePlan)")

	var noPreflight bool
	fs.BoolVar(&noPreflight, "no-preflight", false, "skip the doctor pre-flight check before activation")

	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	profile := common.Profile
	if profile == "" && fs.NArg() > 0 {
		profile = fs.Arg(0)
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "apply: too many arguments; usage: dots apply [profile]")
		return exitcode.Misuse
	}

	p, code := loadOrComputePlan(profile, planPath)
	if code != exitcode.Success {
		return code
	}

	// Already-converged short-circuit: read applied.toml and skip if
	// the saved plan hash matches what we just computed. JSON callers
	// still get a single line on stderr because stdout is reserved
	// for plan/data output.
	if isAlreadyApplied(p) {
		fmt.Fprintf(os.Stderr, "system is already at plan %s; no-op\n", short(p.Hash))
		return exitcode.NoOp
	}

	// --dry-run: render and exit. NoOp if there's literally nothing to
	// do, Success otherwise.
	if common.DryRun {
		emitPlan(p, common)
		if len(p.Steps) == 0 {
			return exitcode.NoOp
		}
		return exitcode.Success
	}

	// Pre-confirm preview. Always rendered to stderr (so JSON consumers
	// of `apply` see a clean stdout) unless --json, in which case we
	// suppress the human view entirely.
	if !common.JSON {
		renderPlan(os.Stderr, p, !common.NoColor)
		fmt.Fprintln(os.Stderr)
	}

	if code := confirmApply(p, common); code != exitcode.Success {
		return code
	}

	// bootstrap-nix is terminal: the just-installed nix is not on this
	// PATH, so the process must exit before any later step runs.
	for _, step := range p.Steps {
		switch step.Kind {
		case plan.KindBootstrapNix:
			if err := bootstrap.InstallNix(os.Stderr, os.Stdin); err != nil {
				fmt.Fprintln(os.Stderr, "apply:", err)
				return exitcode.Failure
			}
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Nix installed. Open a new shell and re-run `dots apply` to continue.")
			return exitcode.Success

		case plan.KindCloneWorkspace:
			path, err := bootstrap.CloneWorkspace(os.Stderr, os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "apply:", err)
				return exitcode.Failure
			}
			if err := os.Chdir(path); err != nil {
				fmt.Fprintln(os.Stderr, "apply: chdir to clone:", err)
				return exitcode.Failure
			}
			workspace.Reset()

		case plan.KindSnapshotConflicts:
			if err := snapshotConflicts(step.Effects); err != nil {
				fmt.Fprintln(os.Stderr, "apply:", err)
				return exitcode.Failure
			}

		case plan.KindApplyProfile:
			if _, err := workspace.Root(); err != nil {
				fmt.Fprintln(os.Stderr, "apply: workspace missing after bootstrap:")
				fmt.Fprintln(os.Stderr, "  what: apply-profile reached without a workspace")
				fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
				fmt.Fprintln(os.Stderr, "  next: rerun `dots apply` from inside the cloned workspace")
				return exitcode.Failure
			}
			if !noPreflight {
				if code := runApplyPreflight(); code != exitcode.Success {
					return code
				}
			}
			if code := runHomeActivation(p.Profile); code != exitcode.Success {
				return code
			}

		default:
			fmt.Fprintf(os.Stderr, "apply: unknown step kind %q (plan schema drift?)\n", step.Kind)
			return exitcode.Failure
		}
	}

	// Success — write the applied receipt. Failures here are reported
	// but do not flip the exit code: the system has converged, and the
	// receipt is for status/diff, not for correctness.
	if path, err := applied.DefaultPath(); err == nil {
		_ = applied.Save(path, applied.State{
			SchemaVersion: applied.SchemaVersion,
			PlanHash:      p.Hash,
			Profile:       p.Profile,
			AppliedAt:     time.Now().UTC(),
		})
	}

	fmt.Fprintf(os.Stderr, "✓ applied plan %s (%d step(s))\n", short(p.Hash), len(p.Steps))
	return exitcode.Success
}

// loadOrComputePlan implements the --plan-file vs. fresh-compute split.
// When --plan FILE is given, the saved plan must match a freshly
// computed one for the same profile; mismatch is a PreFlight failure
// because executing a stale plan against a moved system is the exact
// failure mode that motivated content-addressed plans in the first
// place.
func loadOrComputePlan(profile, planPath string) (plan.Plan, int) {
	fresh, err := computePlan(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply: compute failed:")
		fmt.Fprintln(os.Stderr, "  what: cannot inspect host")
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: rerun with -v to see more, or check $HOME readability")
		return plan.Plan{}, exitcode.Failure
	}
	if planPath == "" {
		return fresh, exitcode.Success
	}
	f, err := os.Open(planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply: open plan file:")
		fmt.Fprintf(os.Stderr, "  what: cannot read %s\n", planPath)
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: check the path or regenerate via `dots plan --out`")
		return plan.Plan{}, exitcode.Failure
	}
	defer f.Close()
	saved, err := plan.Decode(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply: decode plan file:")
		fmt.Fprintf(os.Stderr, "  what: cannot parse %s\n", planPath)
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: regenerate via `dots plan --out`")
		return plan.Plan{}, exitcode.Failure
	}
	if saved.Hash != fresh.Hash {
		fmt.Fprintln(os.Stderr, "apply: plan-vs-system mismatch:")
		fmt.Fprintf(os.Stderr, "  what: saved plan hash %s does not match current host\n", short(saved.Hash))
		fmt.Fprintf(os.Stderr, "  why:  current computePlan yields %s; system state has changed since the plan was saved\n", short(fresh.Hash))
		fmt.Fprintln(os.Stderr, "  next: re-run `dots plan` to refresh, then `dots apply --plan FILE`")
		return plan.Plan{}, exitcode.PreFlight
	}
	// Hashes match by precondition, so saved and fresh describe the
	// same steps. Returning fresh keeps the most up-to-date plan
	// metadata (timestamp, etc.) and avoids carrying around an alias.
	return fresh, exitcode.Success
}

// confirmApply walks the three confirmation modes:
//   - --yes:             skip prompt; print auto-approval line
//   - --non-interactive: refuse to prompt for any consent step;
//     Misuse if the plan needs one
//   - default:           interactive y/N
//
// JSON callers always combine --yes (or --non-interactive) per
// cliflags.Common.Resolve, so the human prompt path never fires under
// --json by construction.
func confirmApply(p plan.Plan, common cliflags.Common) int {
	if common.Yes {
		if !common.JSON {
			fmt.Fprintln(os.Stderr, "auto-approved (--yes)")
		}
		return exitcode.Success
	}
	if common.NonInteractive {
		if needsInteractiveConsent(p) {
			fmt.Fprintln(os.Stderr, "apply: interactive consent required:")
			fmt.Fprintln(os.Stderr, "  what: plan contains a step that mutates the host (Nix install / workspace clone)")
			fmt.Fprintln(os.Stderr, "  why:  --non-interactive forbids prompts and --yes was not given")
			fmt.Fprintln(os.Stderr, "  next: rerun without --non-interactive, or use --yes after reviewing the plan")
			return exitcode.Misuse
		}
		// No consent step required, but still an audit-friendly line.
		if !common.JSON {
			fmt.Fprintln(os.Stderr, "auto-approved (--non-interactive, no consent steps)")
		}
		return exitcode.Success
	}

	fmt.Fprint(os.Stderr, "Proceed? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply: read consent:", err)
		return exitcode.Failure
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(os.Stderr, "declined.")
		return exitcode.Declined
	}
	return exitcode.Success
}

// needsInteractiveConsent flags the step kinds that mutate the host
// outside the workspace and therefore demand an explicit y/N. Steps
// that are confined to the workspace (snapshot, apply-profile) are
// considered already-implicit-in-the-verb.
func needsInteractiveConsent(p plan.Plan) bool {
	for _, s := range p.Steps {
		switch s.Kind {
		case plan.KindBootstrapNix, plan.KindCloneWorkspace:
			return true
		}
	}
	return false
}

// emitPlan respects --json: machine output to stdout, human output to
// stdout (since dry-run IS the result and there's no later side effect
// to keep stdout pristine for).
func emitPlan(p plan.Plan, common cliflags.Common) {
	if common.JSON {
		_ = p.Encode(os.Stdout)
		return
	}
	renderPlan(os.Stdout, p, !common.NoColor)
}

// isAlreadyApplied checks the applied.toml receipt for a hash match.
// A missing or unreadable receipt is treated as "not applied" — the
// safe default.
func isAlreadyApplied(p plan.Plan) bool {
	path, err := applied.DefaultPath()
	if err != nil {
		return false
	}
	st, found, err := applied.Load(path)
	if err != nil || !found {
		return false
	}
	return st.PlanHash != "" && st.PlanHash == p.Hash
}

// snapshotConflicts performs the in-process equivalent of `dots backup`
// for the given relative paths. We do the move ourselves rather than
// calling runBackup so the gum prompt (which would re-confirm) is
// avoided — apply has already obtained consent for the whole plan.
// The backup directory comes from backupSession so the timestamp
// scheme matches what adapters.go uses elsewhere in the binary.
func snapshotConflicts(rels []string) error {
	if len(rels) == 0 {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	dest, err := (&backupSession{}).Dir()
	if err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	for _, rel := range rels {
		src := filepath.Join(home, rel)
		dst := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("create backup parent for %s: %w", rel, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("snapshot %s: %w", rel, err)
		}
	}
	fmt.Fprintf(os.Stderr, "✓ snapshotted %d path(s) → %s\n", len(rels), dest)
	return nil
}

// runHomeActivation execs `nh home switch -c <system> .` directly,
// in-process from dots. The prior `moon run modules:deploy` chain
// added moon's task-graph indirection, argv interpolator, and
// env-passthrough surface area without earning any of moon's caching
// or graph value for a linear two-step sequence (doctor → activate).
//
// `--show-activation-logs` is on by default so any home-manager
// activation-script failure surfaces verbatim. Without it, nh
// reports a bare "Activation failed (exit 1)" and swallows the
// real cause.
func runHomeActivation(profile string) int {
	sys := plan.CurrentHost().NixIdent()
	if sys == "" {
		fmt.Fprintln(os.Stderr, "apply: cannot resolve nix system identifier")
		fmt.Fprintf(os.Stderr, "  what: GOOS=%s GOARCH=%s is not in the supported set\n", runtime.GOOS, runtime.GOARCH)
		fmt.Fprintln(os.Stderr, "  next: dots supports darwin/{arm64,amd64} and linux/{arm64,amd64}")
		return exitcode.Failure
	}

	env := activationEnv()
	nhPath, err := lookPathInEnv("nh", env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply: nh not reachable:")
		fmt.Fprintln(os.Stderr, "  what: nh executable not found")
		fmt.Fprintln(os.Stderr, "  why:  not on PATH and not at <workspace>/.devenv/profile/bin/nh")
		fmt.Fprintln(os.Stderr, "  next: install nh, or activate the dev shell (direnv allow / nix develop)")
		return exitcode.Failure
	}

	cmd := exec.Command(nhPath, "home", "switch",
		"--show-activation-logs",
		"-c", sys, ".",
		"--", "--impure", "--accept-flake-config")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "apply: `nh home switch -c %s .` exited %d\n", sys, ee.ExitCode())
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "apply: nh exec failed:")
		fmt.Fprintln(os.Stderr, "  what: could not execute `nh home switch`")
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: run `dots doctor` to diagnose the toolchain")
		return exitcode.Failure
	}
	return exitcode.Success
}

// lookPathInEnv resolves an executable name against the PATH found in
// the given env slice (NOT this process's PATH). Necessary because
// exec.Command uses the calling process's PATH at construction time;
// when we hand the child a different PATH via cmd.Env, the binary
// lookup must use that PATH explicitly. Returns the absolute path on
// success.
func lookPathInEnv(name string, env []string) (string, error) {
	var pathValue string
	prefix := envPath + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			pathValue = strings.TrimPrefix(e, prefix)
			break
		}
	}
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

// runApplyPreflight is the gate `dots apply` runs before activation.
// It is deliberately narrower than `dots doctor`: doctor validates
// the full dev environment (LSPs, formatters, dev shell tooling);
// preflight only validates what `nh home switch` actually needs.
//
// Why split: when the deploy went through moon, the user had to be
// in the dev shell (otherwise moon was unreachable), so a "full
// dev-shell doctor" was a defensible apply gate. Now activation is
// dots-direct (ADR-0015), so apply can succeed without the dev
// shell — but the full doctor reports failures for tools (gopls,
// treefmt, etc.) that are irrelevant to the activation. Surfacing
// those as PreFlight failures was doctor theater.
//
// What apply needs:
//   - workspace reachable (the flake lives there).
//   - nh executable resolvable via activationEnv's augmented PATH.
//   - nix executable on PATH (nh shells out to it).
//
// The state file is intentionally NOT checked: plan computation
// already handled missing state with a default profile. Doctor
// remains for "is this dev environment properly set up" — a
// separate, manual check the user runs explicitly.
func runApplyPreflight() int {
	var failures []string

	if _, err := workspace.Root(); err != nil {
		failures = append(failures, fmt.Sprintf("workspace: %v", err))
	}

	env := activationEnv()
	if _, err := lookPathInEnv("nh", env); err != nil {
		failures = append(failures, "nh: not on PATH and not at <ws>/.devenv/profile/bin/nh")
	}

	if _, err := exec.LookPath("nix"); err != nil {
		failures = append(failures, "nix: not on PATH")
	}

	if len(failures) == 0 {
		return exitcode.Success
	}

	fmt.Fprintln(os.Stderr, "apply: pre-flight failed:")
	for _, f := range failures {
		fmt.Fprintln(os.Stderr, "  ✗ "+f)
	}
	fmt.Fprintln(os.Stderr, "  next: install the missing tool(s), or run `dots doctor` for full diagnostics")
	fmt.Fprintln(os.Stderr, "  escape: rerun with --no-preflight if you have already validated the toolchain")
	return exitcode.PreFlight
}

// activationEnv returns os.Environ() with two additions: PATH-prepend
// the workspace's .devenv/profile/bin (so nh resolves without direnv
// activation when we're inside a workspace checkout), and
// NH_SHOW_ACTIVATION_LOGS=1 (paired with --show-activation-logs in
// runHomeActivation; either alone is sufficient, both is defense in
// depth against an env-strip layer between dots and nh).
//
// Split into pure (buildActivationEnv) + impure wrapper for unit
// tests; the pure form takes the workspace root as a parameter so
// tests don't need a real .prototools file.
func activationEnv() []string {
	env := os.Environ()
	root, _ := workspace.Root()
	return buildActivationEnv(env, root)
}

// buildActivationEnv is the pure half of activationEnv. workspaceRoot
// may be empty — the call site handles "no workspace" by passing ""
// rather than special-casing in two places.
func buildActivationEnv(env []string, workspaceRoot string) []string {
	env = setEnvKey(env, "NH_SHOW_ACTIVATION_LOGS", "1")
	if workspaceRoot != "" {
		env = prependPathEntries(env, []string{
			filepath.Join(workspaceRoot, ".devenv", "profile", "bin"),
		})
	}
	return env
}

// setEnvKey returns env with key=val present (replaced or appended).
func setEnvKey(env []string, key, val string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

// prependPathEntries prepends extras (in given order) to env's PATH.
// extras may be nil — env is returned unchanged. When env carries no
// PATH, one is synthesized from extras alone.
func prependPathEntries(env, extras []string) []string {
	if len(extras) == 0 {
		return env
	}
	sep := string(os.PathListSeparator)
	prepend := strings.Join(extras, sep)
	pathPrefix := envPath + "="
	for i, e := range env {
		if strings.HasPrefix(e, pathPrefix) {
			env[i] = pathPrefix + prepend + sep + strings.TrimPrefix(e, pathPrefix)
			return env
		}
	}
	return append(env, pathPrefix+prepend)
}

// short renders the first 12 hex chars of a hash for human surfaces.
// Twelve is enough to disambiguate plans on a single host across the
// foreseeable future, and short enough to fit on one line of stderr.
func short(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
