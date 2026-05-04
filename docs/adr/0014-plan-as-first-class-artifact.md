# ADR-0014: Plan as a first-class artifact

`dots` previously had no way to inspect what an apply *would* do without doing it. `dots install` ran the wizard, persisted a profile, and either offered to realize or printed three manual steps; there was no point at which the user (or CI) could say "show me the diff between this profile and the running system, I want to review it before approving." The Terraform/Pulumi/Kubernetes lineage solved this with a `plan` verb whose output is a structured artifact `apply` can consume. This ADR adopts that pattern.

## The contract

`dots plan [profile]` computes what `dots apply` would execute and emits it as JSON (with `--json`) or a human-readable diff (default). The JSON has a stable schema (`internal/plan.SchemaVersion`) and a content hash that covers schema-version, profile, host, and the ordered step list — but not the timestamp, so two computations of the same logical plan over the same host produce the same hash.

`dots apply [--plan FILE]` accepts that JSON. When `--plan FILE` is supplied, `apply` decodes it, recomputes a fresh plan against the current system, and rejects mismatched hashes with exit-code 4 (PreFlight) and a teaching error pointing at `dots plan` to refresh. When `--plan` is omitted, `apply` computes the plan in-process and proceeds with the normal consent flow.

Steps are typed by `Kind`: `bootstrap-nix`, `clone-workspace`, `snapshot-conflicts`, `apply-profile`. Each step that mutates the system carries its `Command` field so the user can read the exact shell invocation before approving — the same Honesty discipline ADR-0010 applies to bootstrap, now extended to every step.

## Why this is worth a verb

Three properties drop out cleanly once `plan` exists as data, not just rendering:

1. **CI review.** A team can require `dots plan --json` output as a PR artifact when a profile changes. Reviewers compare the produced plan against the prior generation's hash before approving the merge. This is impossible when `install` rolls plan and apply into one black box.
2. **Bootstrap is no longer special.** "Install Nix" and "clone the repo" are just steps in the plan, with their commands shown verbatim, gated by the same consent the `apply-profile` step gets. ADR-0012 already removed the standalone-artifact dead-end; this ADR completes the picture by making the unified flow's intermediate state visible and inspectable rather than implicit.
3. **`--dry-run` collapses into `plan`.** Two affordances became one. `dots plan` is what `--dry-run` *should* have been all along — its output is useful, not thrown away.

## Companion: the `applied` state file

`apply` writes a small TOML record at `$DOTS_STATE_HOME/dots/applied.toml` (falling back through `$XDG_STATE_HOME/dots/` and `~/.local/state/dots/`) capturing the just-applied plan's hash, profile, and timestamp. `dots status` reads it and reports the receipt. This file is procedural — it records *what was applied* — and is deliberately distinct from the workspace's `.dots-state.toml`, which is declarative — it records *what should be applied*. The two files have two purposes; ADR-0012 implicitly wanted this split, and this ADR makes it concrete.

The applied-state file is owned exclusively by `dots apply` and `dots rollback`. It is never read by Nix code; it is not part of the realization input. Hand-editing it is harmless — `status` reports stale info, the next apply overwrites it.

## Trade-offs

The plan format is now part of the stable surface. Bumping `plan.SchemaVersion` is a breaking change; the rejection error in `Decode` will surface mismatches loudly so users with stale saved plans know exactly what to do. The hash covers `Steps` but not transient runtime state (e.g., the exact size of a snapshot directory) — two systems with the same persona but different brownfield collisions will produce different plans, and that is correct: the snapshot step's `Effects` field carries the colliding paths and is part of the hash. Alternative considered: hash only the `apply-profile` step (rejected — bootstrap and snapshot are user-visible mutations and belong in the witness).

The plan does not currently track Nix store paths, fixed-output derivations, or flake input pinning. Those live one layer down and `home-manager` evaluation owns them. A future ADR may extend the schema with a `derivations` section if the cost of evaluating it cheaply becomes possible; until then the plan describes what `dots` does, not what `home-manager` will compute downstream of `moon run modules:deploy`.
