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

`dots` is a Nix-native dotfiles platform with two components
([ADR-0001](docs/adr/0001-two-components-one-product.md)):

1. **The TUI binary** (`apps/cli`) — distributed standalone via Homebrew
   tap, GitHub releases (curl-install fetcher), and `nix run`. Runs
   anywhere a Go binary runs. Does not require Nix at runtime. Produces
   `~/.config/dots/selection.toml`.
2. **The realization layer** (`modules/`, `flake.nix`) — a Nix flake plus
   Home Manager (and a reserved nix-darwin slot) that consume
   `selection.toml` and realize the system. Requires Nix and the cloned
   workspace.

Components do not import each other across the split. The TUI knows
nothing about Nix; the realization layer knows nothing about Go source
beyond the schema-contract surface. 

This repository is a VERY EARLY WIP. Proposing sweeping changes that improve long-term maintainability is encouraged.

## Tooling Lanes

Lanes prevent overlap from becoming ambiguity. Each tool owns one
concern.

| Lane | Tool | Owns |
|---|---|---|
| Dotfiles environment | Nix flake | Shell, terminal, editor, multiplexer, system utilities |
| Per-project runtimes | Proto | Go, Bun, Node, language versions; `.prototools`-pinned. Proto's shims are first in `$PATH`. ([ADR-0004](docs/adr/0004-proto-owns-language-runtimes.md)) |
| Build/test/lint DAG | Moon | Go DAG under `apps/` plus repo-root tasks (formatting, repo-wide gates) declared in top-level `moon.yml`. Does not orchestrate Nix. ([ADR-0005](docs/adr/0005-moon-owns-go-dag-never-nix.md)) |
| Dev shell entry | Devenv (via `nix develop`) | Shell package set, treefmt integration, hooks. `languages.*` left empty by design. Direnv activates it. |
| Formatting | treefmt | Multi-language formatter dispatch via `nix fmt`. |
| Realization driver | `nh` (+ `nom` renderer) | `dots deploy` shells out to `nh home switch`; `nom` composed via `$PATH`. ([ADR-0006](docs/adr/0006-nh-deploy-driver-nom-build-renderer.md)) |
| User-facing UX | `dots` binary | Reading and writing `selection.toml`. Never installs packages. Never calls `brew`, `curl \| sh`, or `pip install`. |

`$PATH` priority, top to bottom: Proto shims → nix-darwin / NixOS system
→ Home Manager profile → OS defaults. `dots doctor` verifies this order.

## Maintainability

Long term maintainability is a core priority. If you add new functionality, first check if there is shared logic that can be extracted to a separate module. Duplicate logic across multiple files is a code smell and should be avoided. Don't be afraid to change existing code. Don't take shortcuts by just adding local logic to solve a problem.

## Distribution Model

The two components in §4 produce three entry points. *Entry point* (how
the binary reaches the user) is orthogonal to *depth* (whether the user
only writes a profile or also realizes it).

### Entry points

| Entry point | Command | Delivers |
|---|---|---|
| Homebrew | `brew install sanurb/tap/dots` | Persistent binary |
| Curl install | `curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh \| sh` | Persistent binary in `$INSTALL_DIR` (default `~/.local/bin`) |
| `nix run` | `nix run github:sanurb/.dotfiles -- <args>` | Ephemeral execution |

The Homebrew tap (`sanurb/homebrew-tap`) ships only `dots`. The
curl-install fetcher is POSIX `sh`, always verifies SHA-256, and
verifies the cosign signature when `cosign` is on `$PATH`; manual
fetch + verify is documented in `RELEASING.md`. `nix run` requires Nix
but no clone of the workspace.

### Depth

| Depth | What runs | Realizes system? | Requires Nix? | Requires workspace clone? |
|---|---|---|---|---|
| TUI only | `dots install` | No | No | No |
| Full | `dots install` then `dots deploy` | Yes | Yes (offered on demand) | Yes (offered on demand) |

The TUI's exit screen states which step is done and what remains; the
asymmetry is communicated by binary output, not by gatekeeping channels.
`dots deploy` self-bootstraps both prereqs with explicit per-prereq
consent (ADR-0010); a fresh machine reaches realization through prompts,
not through a copy-and-paste recipe.

Whether the curl-install path stays as-is, gets deprecated, or gets
blessed as a primary path is a **deferred** policy decision (§12) —
this document describes the current state, not the target state.

## Architectural Invariants

Properties that must never break. Each has a verification command.

| # | Invariant | Verification |
|---|---|---|
| I1 | The TUI binary does not import any Nix-related Go modules. Subprocess invocation of sibling subcommands (e.g. `dots install` exec-ing `dots deploy`) and external binaries (`nh`, `moon`, the Determinate Systems installer) is permitted — that is not an import, and the TUI does not encode Nix concepts beyond the existence of the sibling subcommand. ADR-0009 records the distinction. | runnable — see block below |
| I2 | The realization layer references the TUI tree only at the schema-contract surface | runnable — see block below |
| I3 | `selection.toml` schema agrees between Go and Nix consumers | manual: PR review (automated parity check deferred — §12) |
| I4 | `nix develop -c` is the only sanctioned execution surface for dev tooling | manual: PR review |
| I5 | `flake.lock` is never hand-edited | runnable — see block below |
| I6 | Every subcommand declares workspace-required vs workspace-optional | manual: review every new command in `apps/cli/` |
| I7 | Modules under `modules/home/<category>/` are orthogonal — a shell module references no terminal symbols | manual: PR review (automated ortho-check deferred — §12) |

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

## Architectural Decisions

Index of recorded decisions. Each entry links to its ADR file under
`docs/adr/`. Slot numbers are append-only — a vacant slot indicates a
previously dropped decision and is not backfilled. Status defaults to
accepted unless marked otherwise.

| # | ADR |
|---|---|
| 0001 | [Two components, one product](docs/adr/0001-two-components-one-product.md) |
| 0003 | [Home Manager + nix-darwin as the realization layer](docs/adr/0003-home-manager-nix-darwin-realization-layer.md) |
| 0004 | [Proto owns language runtimes](docs/adr/0004-proto-owns-language-runtimes.md) |
| 0005 | [Moon owns the Go DAG; never Nix](docs/adr/0005-moon-owns-go-dag-never-nix.md) |
| 0006 | [`nh` is the deploy driver, `nom` the build renderer](docs/adr/0006-nh-deploy-driver-nom-build-renderer.md) |
| 0007 | [`apps/` directory admission criteria](docs/adr/0007-apps-directory-admission-criteria.md) |
| 0008 | [Per-tool nixpkgs pinning experiment](docs/adr/0008-per-tool-nixpkgs-pinning-experiment.md) |
| 0009 | [The TUI invokes `dots deploy` via subprocess](docs/adr/0009-tui-invokes-deploy-via-subprocess.md) |
| 0010 | [`dots deploy` self-bootstraps Nix and the workspace clone](docs/adr/0010-dots-deploy-self-bootstraps-prereqs.md) |
