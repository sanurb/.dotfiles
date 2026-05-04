# ADR-0013: Verb grammar and exit-code contract

`dots` previously exposed eight verbs (`install`, `sync`, `scan`, `backup`, `deploy`, `doctor`, `version`, `help`) whose names did not partition cleanly along the lines of what they actually did. `install` overloaded "capture a profile" with "run the wizard"; `deploy` connoted pushing to a server when the operation is a local convergence; there was no read-only inspection verb at all. This ADR adopts a designed grammar over the accreted one.

## The grammar

Every verb belongs to one of three groups, reflecting its side-effect class. The group is the contract; the names are servants of the group.

**Group A — measure (read-only):** `status`, `plan`, `diff`, `doctor`, `why`, `explain`. None mutate the system. All accept `--json` for machine output. `plan` is a special case: it is read-only at the OS level (computes what `apply` would do) and produces a serializable artifact downstream verbs consume.

**Group B — converge (state-changing, idempotent):** `init`, `apply`, `update`, `rollback`. These produce a new Home Manager generation or no generation at all. ADR-0010's bootstrap-with-consent contract applies to `init` and `apply`.

**Group C — power-user / composable:** `capture`, `profile`, `completion`. Off the golden path; explicit by design.

`install` and `deploy` are retained as aliases (`install` → `init`, `deploy` → `apply`) so existing scripts, README copy, and the wizard's subprocess invocation (ADR-0009) keep working. Aliases are advertised in `--help` so users see the canonical names; they are not deprecated, they are kept.

## Why `init`, not `install`

`install` collides with package-install semantics across every adjacent tool (`brew install`, `apt install`, `nix profile install`, `pip install`). A user typing `dots install foo` reasonably expects to install package `foo` — a confusion this CLI does not need to host. `init` is the established term for "bring this thing into existence from nothing": `git init`, `npm init`, `cargo init`, `terraform init`, `chezmoi init`. The verb already lives in the user's hand.

## Why `apply`, not `switch`

`apply` is cross-tool and means the same thing in `kubectl apply`, `terraform apply`, `pulumi up`, `chezmoi apply`. `switch` is the Nix-native idiom (`home-manager switch`, `nixos-rebuild switch`) and remains a perfectly defensible alternative. We chose `apply` because the population of users typing this CLI is broader than Nix natives, and the cross-tool word reduces the cognitive cost of one more command to learn. Adding `switch` as a future alias is cheap if requested.

## Exit-code contract

A small, stable, documented set. `internal/exitcode` is the single source of truth.

| Code | Meaning |
|---|---|
| 0 | Success. Also: `--dry-run` over a non-empty plan; `status` with valid output. |
| 1 | Generic runtime failure (subprocess crashed, I/O error, unexpected). |
| 2 | Misuse: unknown verb, bad flag, prompt needed under `--non-interactive`. |
| 3 | User declined a confirmation. Distinct from error so wrapper scripts can branch. |
| 4 | Pre-flight failed: doctor caught drift, plan-vs-system hash mismatch, unmet prereq. |
| 5 | No-op: nothing to do. Useful for `apply --dry-run` in CI guards. |
| 130 | SIGINT / wizard abort. Inherited from POSIX convention. |

`--force` is deliberately absent. If a verb is so dangerous that it needs an escape hatch we redesign the verb. The Honesty contract from ADR-0010 — print the exact command, ask `[y/N]`, default No — is the only way state-changing operations get authorized. `--yes` auto-approves but still prints what it approved.

## Alternatives considered

Renaming everything in one pass without aliases (rejected — breaks ADR-0009's wizard subprocess and every README/script). A larger Cobra-style framework with auto-generated help and per-verb subcommand trees (rejected — stdlib `flag` is sufficient for this surface; the dependency cost and dispatcher rewrite buy us little). Splitting `plan` into `plan` and `show-plan` (rejected — one verb, one job; piping `dots plan --json` through `jq` covers the introspection case).

## Trade-offs

The alias surface (`install`, `deploy`) is now part of the contract. Renaming or removing an alias is a breaking change. We accept this in exchange for not breaking every existing invocation on day one. ADR-0014 records the plan-as-artifact piece of this redesign separately so each ADR has one decision to defend.
