# Proto owns language runtimes; Nix does not

Go, Bun, Node, and per-project language versions are pinned in
`.prototools` and resolved by proto's shims, which sit first on
`$PATH`. Nix manages the dotfiles environment (shells, terminals,
editors, system utilities) but not the language toolchains used to
build `apps/`. Iterating on a Go version under Nix means a flake bump
and a slow rebuild; under proto it is `proto install` and a shim flip.

## Considered Options

- **`nixpkgs.go` / `nixpkgs.bun` / etc.** — rejected for the iteration
  cost on tool versions a developer changes more often than a system
  rebuild can absorb. Reproducibility is preserved by `.prototools`
  pins, which proto enforces per-project.
