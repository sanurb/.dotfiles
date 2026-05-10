# dots explain: generations

dots realizes the workspace by delegating to Nix and Home Manager via the
`nh` driver. Every successful apply writes a new Home Manager _generation_:
an immutable, garbage-collected snapshot of the activated profile.

Why this matters in practice:

- Apply is atomic at the generation boundary. A failed activation does
  not partially overwrite the previous generation; the previous one
  stays current until the new one fully succeeds.
- Generations are listable with `home-manager generations` (or the nh
  equivalent). dots does not own that surface; it inherits it.
- The XDG profile gcroot at
  `~/.local/state/nix/profiles/home-manager`
  points at the current generation. dots doctor reads its presence as
  "Home Manager has activated at least once on this host."

Rollback model:

- `dots rollback` (when implemented) maps to switching the profile
  pointer back to the previous generation. Until then, use the
  Home Manager / nh CLI directly to list and switch generations.
- The Plan that produced a generation is not stored inside the
  generation; the receipt in `applied.toml` is the cross-reference.

This file documents intent. Generations belong to Nix; dots is a polite
caller, not the source of truth for them.
