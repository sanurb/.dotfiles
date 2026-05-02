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

// command pairs a handler with its workspace requirement. The product
// has two distribution stories: the TUI/version/help layer that runs
// anywhere (a Homebrew-shippable binary), and the realization layer
// (deploy/doctor/sync/scan/backup) that consumes the flake at the
// workspace root and is meaningless outside it. The dispatcher uses
// requiresWorkspace to gate the second category with an actionable
// message instead of letting subcommand internals fail opaquely.
type command struct {
	requiresWorkspace bool
	run               func(rest []string) int
}

func commands() map[string]command {
	return map[string]command{
		"install": {requiresWorkspace: false, run: func([]string) int {
			code := ui.Run(ui.ModeInstall, newWizardDeps())
			if code == 0 {
				printInstallNextSteps()
			}
			return code
		}},
		"sync": {requiresWorkspace: true, run: func([]string) int {
			// Reconcile .git/hooks with .moon/workspace.yml's `vcs.hooks`
			// before the activation phase. A brownfield sync is the most
			// likely entry point for a tree whose hooks predate the Moon
			// migration (or were stomped by another tool); running the
			// sync here means the next commit/push goes through the
			// canonical `moon run` gates rather than a stale shim. Errors
			// are non-fatal — see syncMoonHooksSilent for the rationale.
			syncMoonHooksSilent()
			return ui.Run(ui.ModeSync, newWizardDeps())
		}},
		"scan":   {requiresWorkspace: true, run: func([]string) int { return runScan() }},
		"backup": {requiresWorkspace: true, run: func([]string) int { return runBackup(false) }},
		"deploy": {requiresWorkspace: true, run: func([]string) int { return runDeploy() }},
		"doctor": {requiresWorkspace: true, run: func(rest []string) int {
			// Per-subcommand flag set: top-level flag.Parse() stops at the
			// first non-flag arg, so `dots doctor --json` was silently
			// dropping the flag. Local FlagSet captures it correctly.
			fs := flag.NewFlagSet("doctor", flag.ExitOnError)
			jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
			_ = fs.Parse(rest)
			return runDoctor(*jsonOut)
		}},
	}
}

func main() {
	cmds := commands()

	if len(os.Args) < 2 {
		// `dots` with no args opens the install wizard. There is exactly
		// one TUI surface in this binary by design. install is the
		// workspace-optional product surface, so no gate fires here.
		os.Exit(cmds["install"].run(nil))
	}

	name, rest := os.Args[1], os.Args[2:]
	switch name {
	case "-h", "--help", "help":
		fmt.Fprint(os.Stderr, usage)
		os.Exit(0)
	case "-V", "--version", "version":
		fmt.Printf("dots %s (commit %s, built %s, %s/%s)\n",
			version, commit, date, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	c, ok := cmds[name]
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
// next step is `dots apply`-equivalent (`dots deploy` here); outside,
// the user needs Nix and a clone before realization. Reuses the same
// detection helper as the dispatcher gate — one source of truth for
// "is this an install-only host vs a realization host."
func printInstallNextSteps() {
	if _, err := workspace.Root(); err == nil {
		fmt.Println()
		fmt.Println("Profile written. Run `dots deploy` to realize this profile.")
		return
	}
	fmt.Println()
	fmt.Println("Profile written.")
	fmt.Println()
	fmt.Println("To realize this profile, you need the dotfiles workspace and Nix:")
	fmt.Println("  1. Install Nix:    https://determinate.systems/nix-installer")
	fmt.Println("  2. Clone the repo: git clone https://github.com/sanurb/.dotfiles ~/.dotfiles")
	fmt.Println("  3. Realize:        cd ~/.dotfiles && dots deploy")
}

// workspaceRequiredMessage is the user-facing surface for the gate.
// Exit code 2 (misuse / wrong context) and stderr — distinct from a
// runtime error. The message is actionable: clone-and-run, or nix run.
func workspaceRequiredMessage(name string) string {
	return fmt.Sprintf(`dots %s: this command requires a workspace.

Clone the repo and run from inside it:
  git clone https://github.com/sanurb/.dotfiles ~/.dotfiles
  cd ~/.dotfiles
  dots %s

Or run via Nix without a persistent clone:
  nix run github:sanurb/.dotfiles -- %s
`, name, name, name)
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
