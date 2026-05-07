// dots — interactive frontend for the Nix-managed environment.
// The Go binary is the user-facing layer; Moon + Nix are the deterministic
// backend. The CLI never mutates ~/ directly: it scans, prompts, and
// delegates to `moon run modules:<task>`.
//
// The verb grammar groups every subcommand by side-effect class:
// converge (state-changing, idempotent), measure (read-only), power-user
// (composable, off the golden path), and meta. The dispatcher in this file
// owns name resolution, alias lookup, did-you-mean on misses, and the
// workspace-required gate. Per-verb behavior lives in cmd_<name>.go.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/bootstrap"
	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/dym"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// Build-time metadata. GoReleaser injects real values via -ldflags; the
// defaults below mark a non-release build (`go build`, `go run`, IDE) so
// `dots version` is always answerable without a release pipeline.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// User-facing strings used by the dispatcher's workspace-required
// gate. Single source of truth — drift between the dispatcher gate
// and any other surface that mentions the same paths would let a
// renamed repo silently rot in one message but not the other.
const (
	repoCloneURL = "https://github.com/sanurb/.dotfiles"
	repoNixURL   = "github:sanurb/.dotfiles"
	cloneTarget  = "~/.dotfiles"
)

// Group names for the usage table. Stable strings — used as
// section headings in --help output. Order in the help reflects
// importance to a first-time user.
const (
	groupConverge = "converge"
	groupMeasure  = "measure"
	groupPower    = "power"
	groupMeta     = "meta"
)

// command pairs a handler with the dispatcher contract: workspace
// requirement (the dispatcher gate fires before the handler if true),
// the human summary used in --help, the group it belongs to, and any
// aliases. requiresWorkspace=false does NOT mean "ignores workspace";
// some handlers (status, why, profile) do their own workspace probe
// with a verb-appropriate exit code (0 for status's "nothing to
// report", 4 for why/profile's "no workspace to inspect").
type command struct {
	requiresWorkspace bool
	summary           string
	group             string
	aliases           []string
	run               func(rest []string) int
}

// commands is the canonical registry. Aliases are resolved through
// aliasIndex (built at init); direct lookup `commands[name]` returns
// only canonical names.
var commands = map[string]command{
	// Group B — converge (state-changing, idempotent)
	"init": {
		summary: "Bootstrap from anywhere: install Nix, clone repo, run the wizard, apply",
		group:   groupConverge,
		aliases: []string{"install"},
		run:     runInstall,
	},
	"apply": {
		summary: cmdApplySummary,
		group:   groupConverge,
		aliases: []string{"deploy"},
		run:     runApply,
	},
	"update": {
		requiresWorkspace: true,
		summary:           cmdUpdateSummary,
		group:             groupConverge,
		run:               runUpdate,
	},
	"rollback": {
		requiresWorkspace: true,
		summary:           cmdRollbackSummary,
		group:             groupConverge,
		run:               runRollback,
	},
	"sync": {
		requiresWorkspace: true,
		summary:           "Brownfield-safe wizard: scan → snapshot → apply",
		group:             groupConverge,
		run:               runSync,
	},

	// Group A — measure (read-only)
	"status":  {summary: cmdStatusSummary, group: groupMeasure, run: runStatus},
	"plan":    {summary: cmdPlanSummary, group: groupMeasure, run: runPlan},
	"diff":    {summary: cmdDiffSummary, group: groupMeasure, run: runDiff},
	"why":     {summary: cmdWhySummary, group: groupMeasure, run: runWhy},
	"explain": {summary: cmdExplainSummary, group: groupMeasure, run: runExplain},
	"doctor": {
		requiresWorkspace: true,
		summary:           "Validate runtimes, LSPs, formatters; persona unless --binary-only",
		group:             groupMeasure,
		run:               runDoctorCmd,
	},
	"scan": {
		requiresWorkspace: true,
		summary:           "Detect brownfield collisions in $HOME (non-interactive)",
		group:             groupMeasure,
		run:               func([]string) int { return runScan() },
	},

	// Group C — power-user / composable
	"capture":    {summary: cmdCaptureSummary, group: groupPower, run: runCapture},
	"profile":    {summary: cmdProfileSummary, group: groupPower, run: runProfile},
	"completion": {summary: cmdCompletionSummary, group: groupPower, run: runCompletion},
	"backup": {
		requiresWorkspace: true,
		summary:           "Move colliding $HOME files into ~/.dots_backups/<ts> (gum-confirmed)",
		group:             groupPower,
		run:               func([]string) int { return runBackup(false) },
	},
}

// aliasIndex maps every accepted name (canonical + alias) to its
// canonical key in commands. Built once at init so dispatch is a
// single map lookup regardless of which spelling the user typed.
var aliasIndex map[string]string

func init() {
	aliasIndex = make(map[string]string, len(commands)*2)
	for name, c := range commands {
		aliasIndex[name] = name
		for _, a := range c.aliases {
			aliasIndex[a] = name
		}
	}
}

// allKnownNames returns every accepted token (canonical + alias),
// sorted. Used by did-you-mean and the completion fallback.
func allKnownNames() []string {
	out := make([]string, 0, len(aliasIndex)+2)
	for name := range aliasIndex {
		out = append(out, name)
	}
	out = append(out, "version", "help")
	sort.Strings(out)
	return out
}

func runInstall(rest []string) int {
	// One install path. If prereqs (Nix, workspace clone) are missing,
	// they are bootstrapped here behind explicit per-prereq consent —
	// the same Honesty-gated subprocess flow `dots apply` uses. The
	// wizard then runs against a real workspace; the binary no longer
	// ships a "standalone" branch that produces a TOML artifact and
	// three manual steps.
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var common cliflags.Common
	common.Bind(fs)
	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	if _, err := workspace.Root(); err != nil {
		if common.NonInteractive {
			// Bootstrap (Nix install, workspace clone) requires
			// per-prereq consent on stdin. Headless callers must
			// satisfy those prereqs out-of-band — exit 2 matches
			// the cliflags contract: "refuse to prompt; exit 2 if
			// a prompt is needed."
			fmt.Fprintln(os.Stderr, "init: bootstrap requires interactive consent (workspace not found).")
			fmt.Fprintln(os.Stderr, "      Clone the repo and re-run, or drop --non-interactive.")
			return exitcode.Misuse
		}
		if code, ok := bootstrapForInstall(); !ok {
			return code
		}
	}

	root, err := workspace.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return exitcode.Failure
	}
	initial, err := resolveInitialState(root, common.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return exitcode.Misuse
	}
	deps, err := newWizardDeps(initial)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return exitcode.Failure
	}

	if common.NonInteractive {
		return runHeadlessInstall(deps, common)
	}
	return runWithRealize(ui.ModeInstall, deps)
}

// bootstrapForInstall fulfills the missing prereqs for runInstall.
// Returns (exitCode, ok). When ok is true the calling install can
// proceed against the now-present workspace; the workspace cache has
// been reset to reflect the post-chdir cwd.
//
// Nix-just-installed exits cleanly with Success (the new nix is not
// on this process's PATH; user is told to open a new shell and
// re-run). Declined consent maps to Declined (3), not Misuse (2):
// the user knew what they were saying no to.
func bootstrapForInstall() (int, bool) {
	if !bootstrap.NixPresent() {
		consented, err := bootstrap.OfferNixInstall(os.Stdout, os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "init:", err)
			return exitcode.Failure, false
		}
		if !consented {
			return exitcode.Declined, false
		}
		fmt.Println()
		fmt.Println("Nix installed. Open a new shell and re-run `dots init` to continue.")
		return exitcode.Success, false
	}
	path, err := bootstrap.OfferWorkspaceClone(os.Stdout, os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return exitcode.Failure, false
	}
	if path == "" {
		return exitcode.Declined, false
	}
	if err := os.Chdir(path); err != nil {
		fmt.Fprintln(os.Stderr, "init: chdir to clone:", err)
		return exitcode.Failure, false
	}
	workspace.Reset()
	return exitcode.Success, true
}

func runSync(rest []string) int {
	// Reconcile .git/hooks with .moon/workspace.yml's `vcs.hooks` before
	// the activation phase — a brownfield sync is the most likely entry
	// point for a tree whose hooks predate the Moon migration.
	syncMoonHooksSilent()
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var common cliflags.Common
	common.Bind(fs)
	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	root, err := workspace.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		return exitcode.Failure
	}
	initial, err := resolveInitialState(root, common.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		return exitcode.Misuse
	}
	deps, err := newWizardDeps(initial)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		return exitcode.Failure
	}
	return runWithRealize(ui.ModeSync, deps)
}

// runWithRealize runs the wizard and, on consent, hands off to
// `dots apply` as a subprocess.
//
// The subprocess is invoked with --yes because the wizard's realize
// prompt already obtained the user's consent; re-prompting in the
// child would be a regression. The plan and "auto-approved" line
// still print, so the user sees what's running.
func runWithRealize(mode ui.Mode, deps ui.Deps) int {
	r := ui.Run(mode, deps)
	if r.Code != 0 {
		return r.Code
	}
	if !r.RealizeRequested {
		printInstallNextSteps()
		return exitcode.Success
	}
	self, err := resolveSelf()
	if err != nil {
		fmt.Fprintln(os.Stderr, "apply:", err)
		return exitcode.Failure
	}
	return run(self, "apply", "--yes")
}

// resolveSelf locates the running dots binary. os.Executable() is the
// happy path; the LookPath fallback covers layouts where Executable()
// returns a path the child process can't re-exec (some NixOS
// /proc/self/exe cases) or where PATH was mutated between the install
// and apply phases.
func resolveSelf() (string, error) {
	if self, err := os.Executable(); err == nil {
		return self, nil
	}
	self, err := exec.LookPath("dots")
	if err != nil {
		return "", fmt.Errorf("cannot locate dots binary: %w", err)
	}
	return self, nil
}

func runDoctorCmd(rest []string) int {
	// Local FlagSet because top-level flag.Parse stops at the first
	// non-flag arg — `dots doctor --json` would silently drop the flag.
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	binaryOnly := fs.Bool("binary-only", false, "skip persona checks (CI mode — Home Manager activation not assumed)")
	_ = fs.Parse(rest)
	return runDoctor(*jsonOut, *binaryOnly)
}

func main() {
	// Bare `dots` → init wizard. Predictable: typing `dots` on a
	// configured machine still re-runs init, which is idempotent (it
	// detects existing Nix and existing workspace and proceeds to the
	// wizard with no destructive action).
	name := "init"
	var rest []string
	if len(os.Args) >= 2 {
		name, rest = os.Args[1], os.Args[2:]
	}

	switch name {
	case "-h", "--help", "help":
		runHelp(rest)
		return
	case "-V", "--version", "version":
		fmt.Printf("dots %s (commit %s, built %s, %s/%s)\n",
			version, commit, date, runtime.GOOS, runtime.GOARCH)
		return
	}

	canonical, ok := aliasIndex[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "dots: unknown command %q\n", name)
		if suggestion, found := dym.Suggest(name, allKnownNames(), 3); found {
			fmt.Fprintf(os.Stderr, "\nDid you mean `dots %s`?\n", suggestion)
		}
		fmt.Fprintln(os.Stderr, "\nRun `dots help` for the verb list.")
		os.Exit(exitcode.Misuse)
	}
	c := commands[canonical]

	if c.requiresWorkspace {
		if _, err := workspace.Root(); err != nil {
			fmt.Fprint(os.Stderr, workspaceRequiredMessage(canonical))
			os.Exit(exitcode.Misuse)
		}
	}

	os.Exit(c.run(rest))
}

// runHelp renders either the top-level help (no arg) or per-verb help
// (one arg). Per-verb help is intentionally lightweight — it prints
// the summary line and points the user at `<verb> --help` for the
// flag-level surface, since each verb owns its own flag set.
func runHelp(rest []string) {
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, renderUsage())
		return
	}
	name := rest[0]
	canonical, ok := aliasIndex[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "dots help: unknown command %q\n", name)
		if suggestion, found := dym.Suggest(name, allKnownNames(), 3); found {
			fmt.Fprintf(os.Stderr, "Did you mean `%s`?\n", suggestion)
		}
		os.Exit(exitcode.Misuse)
	}
	c := commands[canonical]
	fmt.Fprintf(os.Stderr, "dots %s — %s\n", canonical, c.summary)
	if len(c.aliases) > 0 {
		fmt.Fprintf(os.Stderr, "aliases: %s\n", strings.Join(c.aliases, ", "))
	}
	if c.requiresWorkspace {
		fmt.Fprintln(os.Stderr, "\nRequires a cloned dotfiles workspace.")
	}
	fmt.Fprintf(os.Stderr, "\nFor flags, run: dots %s --help\n", canonical)
}

// printInstallNextSteps tells the user what to do after a wizard run
// where they declined the realize prompt. The wizard always runs
// workspace-backed.
//
// On the realize-yes path, this function isn't called — `dots apply`
// has already run and printed its own outcome by the time control
// returns to main, so any extra epilogue would just be noise.
func printInstallNextSteps() {
	fmt.Println()
	fmt.Println("Profile saved. Run `dots apply` when ready (alias: `dots deploy`).")
}

// workspaceRequiredMessage is the user-facing surface for the gate.
// Exit code Misuse and stderr — distinct from a runtime error. The
// message is actionable: clone-and-run, or nix run.
func workspaceRequiredMessage(name string) string {
	return fmt.Sprintf(`dots %s: this command requires a workspace.

Clone the repo and run from inside it:
  git clone %s %s
  cd %s
  dots %s

Or run via Nix without a persistent clone:
  nix run %s -- %s
`, name, repoCloneURL, cloneTarget, cloneTarget, name, repoNixURL, name)
}

// renderUsage builds the grouped --help output. Order: converge,
// measure, power, meta. Each row is `  dots <name>  <summary>` with
// aliases parenthesized. Width is computed dynamically over the
// longest name in each group.
func renderUsage() string {
	groups := []struct {
		key   string
		title string
	}{
		{groupConverge, "Converge (state-changing, idempotent)"},
		{groupMeasure, "Measure (read-only)"},
		{groupPower, "Power-user / composable"},
	}

	var b strings.Builder
	b.WriteString("dots — Nix-native dotfiles platform\n\n")
	b.WriteString("Usage:\n  dots [verb] [flags]\n\n")
	b.WriteString("With no verb, `dots` runs the init wizard (alias: `dots init`).\n\n")

	for _, g := range groups {
		fmt.Fprintf(&b, "%s:\n", g.title)
		var rows []string
		for name, c := range commands {
			if c.group != g.key {
				continue
			}
			rows = append(rows, name)
		}
		sort.Strings(rows)
		width := longest(rows)
		for _, name := range rows {
			c := commands[name]
			line := fmt.Sprintf("  dots %-*s  %s", width, name, c.summary)
			if len(c.aliases) > 0 {
				line += fmt.Sprintf("  (alias: %s)", strings.Join(c.aliases, ", "))
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Meta:\n")
	b.WriteString("  dots version            Print binary version, commit, build date\n")
	b.WriteString("  dots help [verb]        This help, or per-verb summary\n\n")

	b.WriteString("Common flags (every state-changing verb):\n")
	b.WriteString("  --dry-run, -y/--yes, --non-interactive, --json,\n")
	b.WriteString("  -v/--verbose (repeatable), -q/--quiet, --no-color,\n")
	b.WriteString("  --config PATH, --profile NAME\n\n")

	b.WriteString("Documentation:\n")
	b.WriteString("  dots explain plan         The plan/apply contract\n")
	b.WriteString("  dots explain exit-codes   The stable exit-code table\n")
	b.WriteString("  dots explain bootstrap    What `dots init` does on a fresh machine\n")
	b.WriteString("  dots explain xdg          Where dots reads/writes config + state\n")
	return b.String()
}

func longest(xs []string) int {
	n := 0
	for _, x := range xs {
		if len(x) > n {
			n = len(x)
		}
	}
	return n
}
