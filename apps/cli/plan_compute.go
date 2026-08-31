package main

import (
	"fmt"
	"io"
	"os"

	"github.com/sanurb/.dotfiles/apps/cli/internal/applied"
	"github.com/sanurb/.dotfiles/apps/cli/internal/bootstrap"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// computePlan inspects the current host and returns the steps `apply`
// would execute. It is the single producer of plans so the steps shown
// by `dots plan` are exactly what `dots apply` will run.
//
// Returns an error if the system cannot be inspected (e.g., $HOME
// unreadable). Missing prereqs are not errors — they become steps.
//
// Profile resolution: if the supplied profile is empty, fall back to
// the workspace's `.dots-state.toml` shell pillar (the closest thing
// to a "profile" the schema currently exposes). When neither is
// available — typical on a fresh-host bootstrap before clone —
// Profile is left empty; the plan can still describe the work.
func computePlan(profile string) (plan.Plan, error) {
	resolved := profile
	if resolved == "" {
		resolved = readProfileFromState()
	}

	p := plan.New(resolved)
	stepID := 0
	nextID := func() string {
		stepID++
		return fmt.Sprintf("s%d", stepID)
	}

	if !bootstrap.NixPresent() {
		p.Steps = append(p.Steps, plan.Step{
			ID:      nextID(),
			Kind:    plan.KindBootstrapNix,
			Action:  plan.ActionAdd,
			Summary: "install Nix (Determinate Systems)",
			Command: bootstrap.NixInstallCommand(),
		})
	}

	if _, err := workspace.Root(); err != nil {
		clone, cerr := bootstrap.CloneCommand()
		if cerr != nil {
			return plan.Plan{}, fmt.Errorf("compute clone command: %w", cerr)
		}
		target, terr := bootstrap.Target()
		if terr != nil {
			return plan.Plan{}, fmt.Errorf("compute clone target: %w", terr)
		}
		p.Steps = append(p.Steps, plan.Step{
			ID:      nextID(),
			Kind:    plan.KindCloneWorkspace,
			Action:  plan.ActionAdd,
			Summary: fmt.Sprintf("clone dotfiles workspace → %s", target),
			Command: clone,
		})
	} else {
		// findCollisions is only meaningful inside a workspace — without
		// the Home Manager projection there is nothing to collide with.
		home, err := os.UserHomeDir()
		if err != nil {
			return plan.Plan{}, fmt.Errorf("resolve home: %w", err)
		}
		cs, err := findCollisions(home)
		if err != nil {
			return plan.Plan{}, fmt.Errorf("scan collisions: %w", err)
		}
		if len(cs) > 0 {
			effects := make([]string, len(cs))
			for i, c := range cs {
				effects[i] = c.rel
			}
			p.Steps = append(p.Steps, plan.Step{
				ID:      nextID(),
				Kind:    plan.KindSnapshotConflicts,
				Action:  plan.ActionChange,
				Summary: fmt.Sprintf("quarantine %d path(s) into ~/.dots_backups/<ts>/", len(cs)),
				Effects: effects,
			})
		}
	}

	// Realize the runtime versions declared in .prototools — Go, Bun,
	// Node, Rust, etc. ADR-0008 says proto owns these; "Declare, Don't
	// Script" means the user declares the pin and `dots apply` makes
	// it real. Runtime installation precedes Home Manager because
	// activation hooks may consume proto-managed tools (pi uses bun).
	// workspace.Root() uses .prototools as its marker, so a nil error
	// guarantees the file exists — no separate stat needed.
	if _, err := workspace.Root(); err == nil {
		p.Steps = append(p.Steps, plan.Step{
			ID:      nextID(),
			Kind:    plan.KindInstallRuntimes,
			Action:  plan.ActionChange,
			Summary: "install proto-pinned runtimes from .prototools",
			Command: "proto use",
		})
	}

	// "+" on a fresh host (no prior receipt) so the first apply reads
	// as additive; "~" on subsequent applies. Unreadable receipt is
	// best-effort: prefer "+" so a corrupt file does not block planning.
	action := plan.ActionAdd
	if path, err := applied.DefaultPath(); err == nil {
		if st, found, err := applied.Load(path); err == nil && found && st.PlanHash != "" {
			action = plan.ActionChange
		}
	}
	p.Steps = append(p.Steps, plan.Step{
		ID:      nextID(),
		Kind:    plan.KindApplyProfile,
		Action:  action,
		Summary: fmt.Sprintf("apply profile %s", planProfileLabel(resolved)),
		Command: applyProfileCommand(),
	})

	p.Seal()
	return p, nil
}

// readProfileFromState peeks at the workspace state file (if any) for
// a profile-like value. The state schema doesn't have a "profile"
// concept yet — the closest match is the shell pillar — but we keep
// the lookup central so when a real profile field is added, only this
// function changes.
func readProfileFromState() string {
	root, err := workspace.Root()
	if err != nil {
		return ""
	}
	s, found, err := state.Load(state.Path(root))
	if err != nil || !found {
		return ""
	}
	return s.Pillars.Shell
}

// applyProfileCommand renders the exact subprocess invocation the
// apply-profile step will run, for display in the plan. We compute it
// here so `dots plan` and `dots apply`'s pre-confirm preview agree on
// what the user is approving — no place in the code constructs nh's
// argv differently from this string.
func applyProfileCommand() string {
	sys := plan.CurrentHost().NixIdent()
	if sys == "" {
		sys = "<unknown-system>"
	}
	return fmt.Sprintf("nh home switch --show-activation-logs -c %s . -- --impure --accept-flake-config", sys)
}

// planProfileLabel renders the profile slot of the apply summary so an
// empty profile reads as "(default)" instead of "apply profile " with
// a dangling trailing space. Cosmetic — does not affect the hash.
func planProfileLabel(p string) string {
	if p == "" {
		return "(default)"
	}
	return p
}

// renderPlan writes the human-readable rendering of p to w. Counts
// header, one line per step with action symbol (+/~/-), summary, and
// indented command if present. Color is gated on the `color` arg.
func renderPlan(w io.Writer, p plan.Plan, color bool) {
	add, change, remove := p.Counts()
	fmt.Fprintf(w, "Plan: %s · %s · %s\n",
		paint(color, ansiGreen, fmt.Sprintf("%d add", add)),
		paint(color, ansiYellow, fmt.Sprintf("%d change", change)),
		paint(color, ansiRed, fmt.Sprintf("%d remove", remove)))

	if len(p.Steps) == 0 {
		fmt.Fprintln(w, "  (no steps — system is converged)")
		return
	}

	for _, s := range p.Steps {
		sym := paint(color, actionColor(s.Action), s.Action)
		fmt.Fprintf(w, "  %s %s: %s\n", sym, s.Kind, s.Summary)
		if s.Command != "" {
			fmt.Fprintf(w, "      $ %s\n", s.Command)
		}
		for _, e := range s.Effects {
			fmt.Fprintf(w, "      - %s\n", e)
		}
	}
}

// ANSI color constants. Plain escapes — not lipgloss — because plan
// rendering is non-TUI and the cost of a TTY-detection import isn't
// justified for one verb's output.
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiDim    = "\033[2m"
)

func paint(color bool, code, s string) string {
	if !color {
		return s
	}
	return code + s + ansiReset
}

func actionColor(action string) string {
	switch action {
	case plan.ActionAdd:
		return ansiGreen
	case plan.ActionChange:
		return ansiYellow
	case plan.ActionRemove:
		return ansiRed
	default:
		return ansiDim
	}
}
