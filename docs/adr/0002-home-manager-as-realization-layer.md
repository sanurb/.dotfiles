# Home Manager as the realization layer

`dots` realizes user-level state — packages, services, shells,
editors, XDG dirs — through Home Manager (with a reserved nix-darwin
slot for system-level macOS knobs) rather than a hand-rolled symlink
script. We accept Home Manager's evaluation cost and monolithic-
rebuild model because services, generations/rollback, and once-per-
system option declarations earn the rebuild; live-editable configs
(nvim, window manager) escape via `mkOutOfStoreSymlink` so the
iteration loop is not bound to Nix evaluation.

## Considered Options

- **A symlink-only layer** (per _Use Nix Less_: a small POSIX script
  that links `config/<tool>/` into `$XDG_CONFIG_HOME` and a separate
  `home.packages = [ ... ]` for installs) — rejected for the parts of
  the surface Home Manager actually pays for (services composition,
  generation-based rollback, declared-once-across-systems). Adopted
  _in spirit_ for live-editable configs via `mkOutOfStoreSymlink`.

## Consequences

When Home Manager evaluation gets slow, the lever is to move more
files behind `mkOutOfStoreSymlink`, not to retire the layer.
