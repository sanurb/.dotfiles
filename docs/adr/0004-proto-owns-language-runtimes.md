# ADR-0004: Proto owns language runtimes, not Nix

Per-project language runtimes (Go, Bun, Node, …) are managed by proto via `.prototools`, not by Nix's `languages.*` modules in `devenv.nix`. The `languages.*` block is deliberately left empty: enabling it would double-wrap each runtime (Nix-pinned and proto-pinned) and silently defeat proto's lock-step versioning, which is the reason proto is in the toolchain at all. Nix still owns shell-level packages and dev tooling without a per-project version pin (treefmt, gum, `nh`, …) — the lane is "proto for runtimes, Nix for everything else."
