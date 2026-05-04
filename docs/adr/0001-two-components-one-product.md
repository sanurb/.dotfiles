# Two components, one product

`dots` is a Go CLI (`apps/cli/`) plus a Nix realization layer
(`flake.nix`, `modules/`); neither imports the other in source — the
seam is the on-disk state schema (`.dots-state.toml`) and the CLI's
subprocess invocations of Nix tooling. We pay the duplicated-schema
tax once at the boundary so the CLI can ship standalone via Homebrew,
curl, and `nix run`, and so the realization layer can be consumed by
`nix run` users who never pull the Go binary.

## Considered Options

- **Single Go binary embedding a Nix evaluator** (libnixexpr / cgo) —
  rejected because it ties release engineering to Nix's C++ ABI and
  forfeits `nix run github:…` as an entry point.
- **Nix-only project** (users edit `.nix` files directly; no Go CLI) —
  rejected because the wizard, the plan/apply contract, and the
  brownfield-snapshot UX would have to be expressed as Nix
  evaluations, which is the wrong tool for interactive flows.
