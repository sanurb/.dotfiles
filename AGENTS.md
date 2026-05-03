# AGENTS.md

## Task Completion Requirements

- All of `nix develop -c nix flake check`, `nix develop -c go build ./...`, and `nix develop -c go test ./...` must pass before considering tasks completed.
- NEVER work around `nix develop` failing — surface it as a finding and fix the shell first. The canonical entry point is the canonical entry point.
- NEVER hand-edit generated files (`flake.lock`, Cobra stubs, `teatest` snapshots, formatter output). Regenerate via `nix flake update`, `go generate ./...`, `go test -update`, or `nix fmt`.

## Project Snapshot

`dots` is a Nix-native dotfiles platform with two distinct components:

1. The TUI binary — distributed standalone (Homebrew tap, GitHub releases). Runs anywhere. Does not require Nix at runtime. Subcommands: `install`, `version`, `help`. Produces `~/.config/dots/selection.toml`.
2. The realization layer — a Nix flake + Home Manager modules that consume `selection.toml` and realize the system. Requires the cloned workspace and Nix. Subcommands: `deploy`, `doctor`, `sync`, `scan`, `backup`.

Two components, two distribution models, one product. The split is deliberate and load-bearing — it's the architectural answer to "what is `dots`?" and every decision in the repo should reinforce it.

## Maintainability

Long term maintainability is a core priority. If you add new functionality, first check if there is shared logic that can be extracted to a separate module. Duplicate logic across multiple files is a code smell and should be avoided. Don't be afraid to change existing code. Don't take shortcuts by just adding local logic to solve a problem.

## Core Priorities

Lexicographic. When two conflict, higher wins. Do not weigh them.

1. Honesty. Distribution paths, error messages, and subcommand 
   semantics reflect what the code actually does. No path promises 
   more than it delivers.

2. Reproducibility. Realization is declarative and Nix-resolved 
   end-to-end. No procedural escape hatches.

3. Maintainability. Shared logic is extracted before duplication 
   accumulates. Existing code changes when the change improves the 
   architecture.

4. Velocity. `nix develop -c` enters in under one second after 
   `nix-direnv` warm-up. `dots doctor` completes within its latency 
   budget.

5. Restraint. Subcommands, install paths, and config knobs are 
   added only when justified proportionally to their footprint.

If a tradeoff is required, choose architectural honesty and 
reproducibility over short-term convenience.

## Scope Discipline

Each task has a defined scope. Stay within it. The two exceptions:

1. Strictly mechanical extensions — if completing the task as specified inevitably touches a shared helper that needs a one-line update, do it and note it in the commit body.
2. Small obvious refactors — if you encounter duplicated logic that can be extracted in under ~30 lines and the extraction is unambiguous, do it in a separate commit immediately before or after the task's main commit. Commit message starts with `refactor:` so the change is legible.

Anything beyond these two exceptions is a finding, not work. Surface and stop.

The reason scope discipline matters even when maintainability is a priority: a sweeping refactor silently bundled with feature work is unreviewable. Maintainability gains at the cost of reviewability are a net loss. Refactors that deserve to happen deserve their own scoped task.

## Package Roles

- `apps/cli` — the `dots` Go binary. Cobra-based CLI, Bubbletea TUI for `dots install`. Workspace-optional subcommands (TUI, version, help) run anywhere. Workspace-required subcommands (deploy, doctor, sync, scan, backup) check for workspace presence and exit with an actionable message if absent.

## Tooling Lanes

The stack has overlap between tools. The lanes prevent that overlap from becoming ambiguity:

- Nix flake owns the dotfiles environment: shell, terminal, editor, multiplexer, system utilities, and any language runtime the dotfiles themselves depend on (e.g., the Go used to build `dots`). Pinned in `flake.lock`.
- Proto owns per-project language runtimes (Node, Bun, Deno, project-specific Go/Rust versions) configured via `.prototools` files in projects *outside* this repo. Proto's shims must be first in `$PATH`.
- Moon owns the Go CLI's build/test/lint DAG inside `apps/cli` and similar future packages. Does not orchestrate Nix.
- `dots` binary is the user-facing UX layer. Never a package installer. Never calls `brew`, `curl | sh`, or `pip install`.

The `$PATH` boundary, top to bottom in priority: Proto shims → nix-darwin / NixOS system → Home Manager profile → OS defaults. Verifying this order is part of `dots doctor`.

## Distribution Model

Three install paths, in this order:

1. Try the TUI — `brew install sanurb/tap/dots` then `dots install`. Produces `selection.toml`. Realization requires Path 2 or 3.
2. Full install — Path 1 + clone the workspace + `dots deploy`.
3. Nix-native — `nix run github:sanurb/.dotfiles -- install` then `nix run github:sanurb/.dotfiles -- deploy`. No Homebrew.

Paths 2 and 3 require Nix as a hard prerequisite.

The Homebrew tap (`sanurb/homebrew-tap`) ships only `dots`. Do not publish self-installer scripts, curl-installable bundles, or anything that creates the expectation of a self-contained installer. The TUI is genuinely useful standalone (it produces `selection.toml`); the *system configuration* is not, and the distribution must reflect that asymmetry.

## References

External writing that informs the architecture of this project. 
Each entry includes the position it represents and how this 
project relates to it.

### On scope of Nix

- [You don't have to use Nix to manage your dotfiles](https://jade.fyi/blog/use-nix-less/) 
  — argues that Nix earns its place when it's solving the 
  hermetic-package-management problem and gets in the way when 
  stretched beyond it. This project takes that position 
  seriously: Nix realizes the system; it does not distribute 
  the TUI, manage per-project runtimes (Proto does), or wrap 
  the Go build (Moon does). The lanes are deliberate.

### On flake structure

- [Refactoring my infrastructure as code configurations](https://not-a-number.io/2025/refactoring-my-infrastructure-as-code-configurations/#flipping-the-configuration-matrix) 
  — describes the dendritic / modular flake-parts approach.
- [Dendrix](https://dendrix.oeiuwq.com/Dendritic.html) — 
  framework articulation of the same approach.
