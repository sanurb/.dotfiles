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
)

// Build-time metadata. GoReleaser injects real values via -ldflags; the
// defaults below mark a non-release build (`go build`, `go run`, IDE) so
// `dots version` is always answerable without a release pipeline.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		// `dots` with no args opens the install wizard. There is exactly
		// one TUI surface in this binary by design.
		os.Exit(ui.Run(ui.ModeInstall, newWizardDeps()))
	}

	cmd, rest := os.Args[1], os.Args[2:]
	switch cmd {
	case "-h", "--help", "help":
		fmt.Fprint(os.Stderr, usage)
		os.Exit(0)
	case "-V", "--version", "version":
		fmt.Printf("dots %s (commit %s, built %s, %s/%s)\n",
			version, commit, date, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	case "install":
		os.Exit(ui.Run(ui.ModeInstall, newWizardDeps()))
	case "sync":
		// Reconcile .git/hooks with .moon/workspace.yml's `vcs.hooks`
		// before the activation phase. A brownfield sync is the most
		// likely entry point for a tree whose hooks predate the Moon
		// migration (or were stomped by another tool); running the
		// sync here means the next commit/push goes through the
		// canonical `moon run` gates rather than a stale shim. Errors
		// are non-fatal — see syncMoonHooksSilent for the rationale.
		// Formatting drift is no longer healed here: the pre-commit
		// hook is the auto-heal edge, and the doctor gate on deploy
		// still reports any drift that slips through.
		syncMoonHooksSilent()
		os.Exit(ui.Run(ui.ModeSync, newWizardDeps()))
	case "scan":
		os.Exit(runScan())
	case "backup":
		os.Exit(runBackup(false))
	case "deploy":
		os.Exit(runDeploy())
	case "doctor":
		// Per-subcommand flag set: top-level flag.Parse() stops at the
		// first non-flag arg, so `dots doctor --json` was silently
		// dropping the flag. Local FlagSet captures it correctly.
		fs := flag.NewFlagSet("doctor", flag.ExitOnError)
		jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
		_ = fs.Parse(rest)
		os.Exit(runDoctor(*jsonOut))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		os.Exit(2)
	}
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

Non-interactive automation: prefer moon directly (moon run dotfiles:deploy).
The deploy task is gated on cli:check, which runs the doctor — drift fails
the deploy.
`
