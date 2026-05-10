# AGENTS.md

This document defines what `dots` is, how it is structured, and the rules
that govern changes to it. It has two audiences: humans onboarding to the
repo, and agents (LLMs, CI checks, future contributors) who must follow
the rules without rediscovering them. Both audiences must come away with
the same model.

## Task Completion Requirements

- All of `nix develop -c nix flake check`,
  `nix develop -c go build ./apps/cli/...`, and
  `nix develop -c go test ./apps/cli/...` must pass before considering
  tasks completed.
- NEVER work around `nix develop` failing — surface it as a finding and
  fix the shell first. The canonical entry point is the canonical entry
  point.
- NEVER hand-edit generated files (`flake.lock`, Cobra stubs, `teatest`
  snapshots, formatter output). Regenerate via `nix flake update`,
  `go generate ./...`, `go test -update`, or `nix fmt`.

## Scope Discipline

Each task has a defined scope. Stay within it. Two exceptions only:

1. **Strictly mechanical extensions.** If completing the task inevitably
   touches a shared helper that needs a one-line update, do it and note
   it in the commit body.
2. **Small obvious refactors.** If you encounter duplicated logic
   extractable in ≤30 lines unambiguously, extract it in a separate
   commit immediately before or after the task's main commit. The
   commit message starts with `refactor:`.

Anything beyond these is a finding, not work. Surface and stop.

A sweeping refactor silently bundled with feature work is unreviewable.
Maintainability gains at the cost of reviewability are a net loss.

## Project Snapshot

`dots` is a Nix-native dotfiles platform with two components:

1. **The TUI binary** (`apps/cli`) — distributed standalone via Homebrew
   tap, GitHub releases (curl-install fetcher), and `nix run`. Runs
   anywhere a Go binary runs. On a machine missing Nix or the workspace
   clone it self-bootstraps both with explicit per-prereq consent
   rather than producing a portable-profile artifact and stopping.
   Writes the workspace state file (`.dots-state.toml`).
2. **The realization layer** (`modules/`, `flake.nix`) — a Nix flake plus
   Home Manager (and a reserved nix-darwin slot) that consume
   `.dots-state.toml` and realize the system. Requires Nix and the
   cloned workspace.

Components do not import each other across the split. The TUI knows
nothing about Nix; the realization layer knows nothing about Go source
beyond the schema-contract surface.

This repository is a VERY EARLY WIP. Proposing sweeping changes that improve long-term maintainability is encouraged.

## Tooling Lanes

Lanes prevent overlap from becoming ambiguity. Each tool owns one
concern.

| Lane                 | Tool                       | Owns                                                                                                                                                                  |
| -------------------- | -------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Dotfiles environment | Nix flake                  | Shell, terminal, editor, multiplexer, system utilities                                                                                                                |
| Per-project runtimes | Proto                      | Go, Bun, Node, language versions; `.prototools`-pinned. Proto's shims are first in `$PATH`.                                                                           |
| Build/test/lint DAG  | Moon                       | Go DAG under `apps/` plus repo-root tasks (formatting, repo-wide gates) declared in top-level `moon.yml`. Does not orchestrate Nix.                                   |
| Dev shell entry      | Devenv (via `nix develop`) | Shell package set, treefmt integration, hooks. `languages.*` left empty by design. Direnv activates it.                                                               |
| Formatting           | treefmt                    | Multi-language formatter dispatch via `nix fmt`.                                                                                                                      |
| Realization driver   | `nh` (+ `nom` renderer)    | `dots apply` (alias `deploy`) shells out to `nh home switch` directly; `nom` composed via `$PATH`.                                                                    |
| User-facing UX       | `dots` binary              | Reading and writing `.dots-state.toml`. Bootstraps Nix and the workspace clone behind explicit consent. Never installs packages, never calls `brew` or `pip install`. |

`$PATH` priority, top to bottom: Proto shims → nix-darwin / NixOS system
→ Home Manager profile → OS defaults. `dots doctor` verifies this order.

## Maintainability

Long term maintainability is a core priority. If you add new functionality, first check if there is shared logic that can be extracted to a separate module. Duplicate logic across multiple files is a code smell and should be avoided. Don't be afraid to change existing code. Don't take shortcuts by just adding local logic to solve a problem.

## Distribution Model

The two components in §4 produce three peer entry points. _Entry point_
(how the binary reaches the user) is orthogonal to _depth_ (whether the
user only writes a profile or also realizes it). All three entry points
are equally supported; the user picks based on which package
manager they already use, not on a primary/fallback hierarchy.

**Supported platforms.** macOS and Linux only. `nh` and Moon do not
have Windows ports; the TUI binary follows suit and does not ship a
Windows artifact. Windows users use WSL2.

### Entry points

| Entry point  | Command                                                                                       | Delivers                                                     |
| ------------ | --------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| Homebrew     | `brew install sanurb/tap/dots`                                                                | Persistent binary                                            |
| Curl install | `curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh \| sh` | Persistent binary in `$INSTALL_DIR` (default `~/.local/bin`) |
| `nix run`    | `nix run github:sanurb/.dotfiles -- <args>`                                                   | Ephemeral execution                                          |

The Homebrew tap (`sanurb/homebrew-tap`) ships only `dots`. The curl
fetcher downloads the stable raw-binary asset
(`dots-{darwin,linux}-{amd64,arm64}` at
`/releases/latest/download/`), always verifies SHA-256, and verifies
the cosign keyless OIDC signature when `cosign` is on `$PATH`; manual
fetch + verify is documented in `RELEASING.md`. `nix run` requires
Nix but no clone of the workspace.

### Depth

Depth is one path: `dots init` (or its `dots install` alias, or bare
`dots`) takes the user from a fresh machine to a configured system in a
single invocation, gated only by per-prereq `[y/N]` prompts. The wizard
captures the profile, the post-wizard handoff calls `dots apply --yes`
as a subprocess, and apply walks the plan: install Nix → clone
workspace → snapshot conflicts → realize. No "capture a profile and
then do these three things by hand" intermediate.

| Verb (canonical)                                                  | Aliases                | Side-effect class                             |
| ----------------------------------------------------------------- | ---------------------- | --------------------------------------------- |
| `dots init`                                                       | `install`, bare `dots` | converge — bootstrap, wizard, apply           |
| `dots apply`                                                      | `deploy`               | converge — realize the plan                   |
| `dots update`                                                     | —                      | converge — `git pull --ff-only && dots apply` |
| `dots rollback`                                                   | —                      | converge — switch Home Manager generation     |
| `dots sync`                                                       | —                      | converge — brownfield-safe wizard             |
| `dots status`, `plan`, `diff`, `doctor`, `why`, `explain`, `scan` | —                      | measure (read-only)                           |
| `dots capture`, `profile`, `completion`, `backup`                 | —                      | power-user / composable                       |
| `dots version`, `help`                                            | —                      | meta                                          |

`dots help <verb>` prints the per-verb summary; `dots explain <topic>`
is the built-in topic browser shipped inside the binary. The verb
grammar, the stable exit-code table, and the plan-as-artifact contract
are reachable via `dots explain plan` and `dots explain exit-codes`.

## Architectural Invariants

Properties that must never break. Each has a verification command.

| #  | Invariant                                                                                                                                                                                                                                                                                                                                                   | Verification                                              |
| -- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| I1 | The TUI binary does not import any Nix-related Go modules. Subprocess invocation of sibling subcommands (e.g. `dots install` exec-ing `dots deploy`) and external binaries (`nh`, `moon`, the Determinate Systems installer) is permitted — that is not an import, and the TUI does not encode Nix concepts beyond the existence of the sibling subcommand. | runnable — see block below                                |
| I2 | The realization layer references the TUI tree only at the schema-contract surface                                                                                                                                                                                                                                                                           | runnable — see block below                                |
| I3 | `.dots-state.toml` schema agrees between Go and Nix consumers                                                                                                                                                                                                                                                                                               | manual: PR review (automated parity check deferred — §12) |
| I4 | `nix develop -c` is the only sanctioned execution surface for dev tooling                                                                                                                                                                                                                                                                                   | manual: PR review                                         |
| I5 | `flake.lock` is never hand-edited                                                                                                                                                                                                                                                                                                                           | runnable — see block below                                |
| I6 | Every subcommand declares workspace-required vs workspace-optional                                                                                                                                                                                                                                                                                          | manual: review every new command in `apps/cli/`           |
| I7 | Modules under `modules/home/<category>/` are orthogonal — a shell module references no terminal symbols                                                                                                                                                                                                                                                     | manual: PR review (automated ortho-check deferred — §12)  |

Runnable verifications (copy-paste from inside `nix develop`):

```sh
# I1 — TUI's import graph is Nix-free. Tests imports, not strings;
# comments and exec-LookPath calls to `nh`, `moon`, or sibling
# subcommands are out of scope. The path-component anchors avoid
# matching the Go stdlib's `internal/syscall/unix` and
# `golang.org/x/sys/unix`, which legitimately substring-match `nix`.
go list -deps ./apps/cli/... |
  grep -iE '(^|/)(nix|nixpkgs|home-manager|home_manager)(/|$)'   # expect: empty

# I2 — only the schema-contract file is read across the split.
grep -rE 'apps/cli' modules/ flake.nix | grep -v SCHEMA_VERSION   # expect: empty

# I5 — flake.lock diffs look like `nix flake update` output.
git log -p flake.lock   # manual visual check
```
