// dots — interactive frontend for the Nix-managed environment.
// The Go binary is the user-facing layer; Moon + Nix are the deterministic
// backend. The CLI never mutates ~/ directly: it scans, prompts, and
// delegates to `moon run modules:<task>`.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"

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

// User-facing strings reused by both the dispatcher gate and the
// install next-step message. Single source of truth — drift between
// the two would let a renamed repo or installer URL silently rot in
// one message but not the other.
const (
	repoCloneURL = "https://github.com/sanurb/.dotfiles"
	repoNixURL   = "github:sanurb/.dotfiles"
	cloneTarget  = "~/.dotfiles"
	nixInstaller = "https://determinate.systems/nix-installer"
)

// command pairs a handler with the dispatcher's workspace contract.
// requiresWorkspace=true makes the gate fire with workspaceRequiredMessage
// before the handler runs; the handler can assume workspace.Root() succeeds.
type command struct {
	requiresWorkspace bool
	run               func(rest []string) int
}

var commands = map[string]command{
	"install": {requiresWorkspace: false, run: runInstall},
	"sync":    {requiresWorkspace: true, run: runSync},
	"scan":    {requiresWorkspace: true, run: func([]string) int { return runScan() }},
	"backup":  {requiresWorkspace: true, run: func([]string) int { return runBackup(false) }},
	"deploy":  {requiresWorkspace: true, run: func([]string) int { return runDeploy() }},
	"doctor":  {requiresWorkspace: true, run: runDoctorCmd},
}

func runInstall([]string) int {
	// Two-mode install: inside a workspace, the full wizard (pillar
	// selection → scan → optional snapshot → "Realize now?" prompt).
	// Outside, a form-only path that writes the profile to the user-
	// config dir and stops there. Realization is never run from inside
	// the wizard — see ADR-0009 and runWithRealize below.
	if _, err := workspace.Root(); err != nil {
		code := runStandaloneInstall()
		if code == 0 {
			printInstallNextSteps(false)
		}
		return code
	}
	deps, err := newWizardDeps()
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}
	return runWithRealize(ui.ModeInstall, deps)
}

func runSync([]string) int {
	// Reconcile .git/hooks with .moon/workspace.yml's `vcs.hooks` before
	// the activation phase — a brownfield sync is the most likely entry
	// point for a tree whose hooks predate the Moon migration.
	syncMoonHooksSilent()
	deps, err := newWizardDeps()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		return 1
	}
	return runWithRealize(ui.ModeSync, deps)
}

// runWithRealize runs the wizard and, on consent, hands off to
// `dots deploy` as a subprocess. ADR-0009 records the rationale.
func runWithRealize(mode ui.Mode, deps ui.Deps) int {
	r := ui.Run(mode, deps)
	if r.Code != 0 {
		return r.Code
	}
	if !r.RealizeRequested {
		printInstallNextSteps(true)
		return 0
	}
	self, err := resolveSelf()
	if err != nil {
		fmt.Fprintln(os.Stderr, "deploy:", err)
		return 1
	}
	return run(self, "deploy")
}

// resolveSelf locates the running dots binary. os.Executable() is the
// happy path; the LookPath fallback covers layouts where Executable()
// returns a path the child process can't re-exec (some NixOS
// /proc/self/exe cases) or where PATH was mutated between the install
// and deploy phases.
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
	// Default to the install wizard so `dots` (no args) is a single
	// dispatch path, not a special-case branch that has to be kept in
	// sync with the install handler.
	name := "install"
	var rest []string
	if len(os.Args) >= 2 {
		name, rest = os.Args[1], os.Args[2:]
	}

	switch name {
	case "-h", "--help", "help":
		fmt.Fprint(os.Stderr, usage)
		os.Exit(0)
	case "-V", "--version", "version":
		fmt.Printf("dots %s (commit %s, built %s, %s/%s)\n",
			version, commit, date, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	c, ok := commands[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", name, usage)
		os.Exit(2)
	}

	if c.requiresWorkspace {
		if _, err := workspace.Root(); err != nil {
			fmt.Fprint(os.Stderr, workspaceRequiredMessage(name))
			os.Exit(2)
		}
	}

	os.Exit(c.run(rest))
}

// printInstallNextSteps tells the user what to do after a successful
// install wizard run that DID NOT realize. The hasWorkspace flag
// distinguishes the two cases: inside the workspace, "deferred — run
// dots deploy" is enough; outside, the user needs Nix and a clone
// first, so we keep the bootstrap recipe.
//
// On the realize-yes path, this function isn't called — `dots deploy`
// has already run and printed its own outcome by the time control
// returns to main, so any extra epilogue would just be noise.
func printInstallNextSteps(hasWorkspace bool) {
	if hasWorkspace {
		fmt.Println()
		fmt.Println("Profile saved. Run `dots deploy` when ready.")
		return
	}
	fmt.Printf(`
Profile saved.

To realize this profile, you need the dotfiles workspace and Nix:
  1. Install Nix:    %s
  2. Clone the repo: git clone %s %s
  3. Realize:        cd %s && dots deploy
`, nixInstaller, repoCloneURL, cloneTarget, cloneTarget)
}

// workspaceRequiredMessage is the user-facing surface for the gate.
// Exit code 2 (misuse / wrong context) and stderr — distinct from a
// runtime error. The message is actionable: clone-and-run, or nix run.
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

const usage = `dots

Usage:
  dots                       Open the install wizard (alias for 'dots install')
  dots install               Multi-step wizard: capabilities → conflicts → deploy
  dots sync                  Brownfield-safe wizard: conflicts → deploy
  dots scan                  Detect brownfield collisions in $HOME (non-interactive)
  dots backup                Move colliding files into ~/.dots_backups/<ts> (gum-confirmed)
  dots deploy                moon run modules:deploy (no wizard)
  dots doctor [--json] [--binary-only]
                             Validate runtimes, LSPs, formatters; persona
                             unless --binary-only (CI mode)
  dots version               Print binary version, commit, build date

install/version/help run anywhere. The remaining subcommands realize the
workspace and require a clone of the dotfiles repo + Nix on PATH; outside
a workspace they exit with an actionable message and code 2.

Non-interactive automation: prefer moon directly (moon run modules:deploy).
Moon task split: cli:check runs 'dots doctor --binary-only' as a CI gate;
cli:doctor runs the full doctor and gates modules:deploy. Drift in either
fails its respective pipeline.
`
