// dots — interactive frontend for the Nix-managed environment.
// The Go binary is the user-facing layer; Moon + Nix are the deterministic
// backend. The CLI never mutates ~/ directly: it scans, prompts, and
// delegates to `moon run dotfiles:<task>`.
package main

import (
	"flag"
	"fmt"
	"os"
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
	// Two-mode install: inside a workspace, the full wizard (form →
	// scan → snapshot → realize). Outside, a form-only path that
	// writes the profile to the user-config dir and stops there.
	if _, err := workspace.Root(); err != nil {
		code := runStandaloneInstall()
		if code == 0 {
			printInstallNextSteps()
		}
		return code
	}
	deps, err := newWizardDeps()
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}
	code := ui.Run(ui.ModeInstall, deps)
	if code == 0 {
		printInstallNextSteps()
	}
	return code
}

func runSync([]string) int {
	// Reconcile .git/hooks with .moon/workspace.yml's `vcs.hooks` before
	// the activation phase — a brownfield sync is the most likely entry
	// point for a tree whose hooks predate the Moon migration.
	syncMoonHooksSilent()
	// Gate verified workspace presence; defense in depth in case a
	// future change lets workspace.Root() fail post-gate.
	deps, err := newWizardDeps()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sync:", err)
		return 1
	}
	return ui.Run(ui.ModeSync, deps)
}

func runDoctorCmd(rest []string) int {
	// Local FlagSet because top-level flag.Parse stops at the first
	// non-flag arg — `dots doctor --json` would silently drop the flag.
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	_ = fs.Parse(rest)
	return runDoctor(*jsonOut)
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
// install wizard. Branches on workspace presence: in-workspace, the
// next step is realize via `dots deploy`; outside, the user needs Nix
// and a clone before realization. Shares URL constants with the
// dispatcher gate so the two messages can't drift.
func printInstallNextSteps() {
	if _, err := workspace.Root(); err == nil {
		fmt.Println()
		fmt.Println("Profile written. Run `dots deploy` to realize this profile.")
		return
	}
	fmt.Printf(`
Profile written.

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
  dots deploy                moon run dotfiles:deploy (no wizard)
  dots doctor [--json]       Validate every pinned runtime + LSP
  dots version               Print binary version, commit, build date

install/version/help run anywhere. The remaining subcommands realize the
workspace and require a clone of the dotfiles repo + Nix on PATH; outside
a workspace they exit with an actionable message and code 2.

Non-interactive automation: prefer moon directly (moon run dotfiles:deploy).
The deploy task is gated on cli:check, which runs the doctor — drift fails
the deploy.
`
