# Maintenance log

Architectural decisions in this repo intentionally trade complexity for
control — per-tool nixpkgs pins, schema-parity checks, the auto-discover
walk. Each one earns its place by making something concrete better. This
log is how we tell whether they're still earning it.

If a complexity has nothing to log here for ≥ 90 days, the working
hypothesis is that it stopped earning its weight. Open a `refactor:` PR
to collapse it back to the simpler form, citing the empty stretch in
the body.

## Per-tool nixpkgs pins

`flake.nix` exposes `pkgsPins.<name>` and threads it through Home
Manager `extraSpecialArgs` so individual modules can pull from a
different nixpkgs hash than the default. See `flake.nix:inputs` for the
declared pins and `modules/home/editor.nix` for the canonical use site.

The list below tracks **why** each pin exists (which incident or
upstream divergence forced it) and **when** the divergence stopped, so
the pin can be removed and the module collapsed back to the default
`pkgs.<attr>`.

### Active pins

| Pin    | Module                             | Started    | Reason                                                                                                                                                             | Last upstream check  |
| ------ | ---------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------- |
| `edge` | `modules/home/editor.nix` (neovim) | 2026-05-03 | Demonstrate the per-tool-pin pattern; neovim cycles faster than `nixos-unstable`'s hydra gating and the lag has bitten plugin/treesitter parser compat in the past | 2026-05-03 (initial) |

### Retired pins

_(empty — no pin has been collapsed yet)_

### When to retire

A pin retires when the divergence it tracks goes away. Concretely:

1. Pull `nixpkgs` and `nixpkgs-<pin>` into a scratch shell, diff
   `nix eval .#packages.${system}.<attr>.version` for the pinned
   attribute(s).
2. If the versions match for ≥ 4 consecutive weekly checks, the pin is
   no longer providing newer bits — collapse it.
3. If the pin's stated reason was a bug fix, check whether the fix
   landed in the default channel; if so, retire even if versions still
   differ slightly.

Record retirements with the date, the reason ("upstream caught up",
"bug fixed in nixpkgs#NNNNN", etc.), and the commit that removed the
pin. The retired-pins table is the institutional memory; keep it.

## Schema parity (Go ↔ Nix)

_(seeded for PR 4 — populated when the schema-parity check is wired)_

## Flake-show snapshot

_(seeded for PR 4 — populated when the snapshot diff gate is wired)_
