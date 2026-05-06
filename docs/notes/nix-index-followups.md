# nix-index follow-ups

Loose ends from wiring `programs.nix-index` into foundation. Surfaced
here, not fixed inline, so the database-landing commit stays atomic.

## 1. Per-shell integration toggles are implicit, not explicit

`programs.nix-index.enable{Bash,Fish,Nushell,Zsh}Integration` all
default to `true` when `programs.nix-index.enable = true`. That means
the command-not-found hook works in every supported shell out of the
box — no per-shell module needs to flip anything for the feature to
land.

But the rest of the foundation/per-shell boundary is explicit: every
per-shell module under `modules/home/shells/` flips
`programs.{atuin,zoxide,starship}.enable<Shell>Integration` itself,
even though those toggles also default to true on enabled shells.
The redundancy is the documentation — a maintainer reading
`shells/fish.nix` learns from a one-line declaration that fish wires
atuin, without having to grep upstream HM defaults.

Mirroring that pattern for nix-index would mean adding:

- `modules/home/shells/fish.nix` → `programs.nix-index.enableFishIntegration = true;`
- `modules/home/shells/zsh.nix` → `programs.nix-index.enableZshIntegration = true;`
- `modules/home/shells/nushell.nix` → `programs.nix-index.enableNushellIntegration = true;`

Pure consistency change — no behavioral effect. Land in a separate
commit if the boundary discipline is worth the diff.

## 2. Bash has no per-shell module

Bash lives in `modules/home/foundation.nix` (`programs.bash` block),
not under `modules/home/shells/`. The same architectural choice that
makes atuin's bash boundary implicit applies to nix-index — bash gets
the integration via the module's default `enableBashIntegration =
true` and there's no per-shell module to flip it explicitly.

If bash ever grows a `modules/home/shells/bash.nix` (the same way
zsh did once Powerlevel10k opinions arrived), the nix-index toggle
should move there alongside the atuin/zoxide ones. Until then,
foundation owns it implicitly and that's the documented exception.

## 3. Database staleness

`nix-index-database` rebuilds nightly upstream. `flake.lock` pins
this repo to whichever rev was current at last `nix flake update`,
so suggestions for very-recently-added nixpkgs derivations may miss
until the lock is refreshed. Not actionable here; flagged so the
"why doesn't `, foo` find foo when nix has it?" question doesn't get
debugged from scratch.
