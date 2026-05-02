# dots

Interactive frontend for a Nix-managed personal environment. The Go binary is
the user-facing layer; Moon + Nix + Home Manager are the deterministic
backend. `dots` never mutates `~/` directly — it scans, prompts, and delegates
to `moon run dotfiles:<task>`.

## Installation

`dots` supports two installation paths with different guarantees. Pick the
one that matches your environment.

| Path | Source | Linking | Reproducibility |
|------|--------|---------|-----------------|
| Pre-built binary (Homebrew, install script) | GoReleaser CI | Dynamically linked against system libs | Per-release pinned via signed checksums |
| Nix flake | Built from source on your machine | Linked against the Nix store | Fully hermetic, integrates with the rest of the dotfiles flake |

**Recommendation.** Use the pre-built binary if you just want to try `dots` or
do not have Nix installed. Use the Nix flake if you want the full `dots`
experience as designed — the binary, the Home Manager modules, and the
declared toolchain all from one source of truth.

### Homebrew

```sh
brew install sanurb/tap/dots
```

This installs the latest release from the [`sanurb/homebrew-tap`][tap] tap.
Updates land via `brew upgrade dots`.

### Install script

```sh
curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh | sh
```

The script:

- Detects your OS and architecture.
- Downloads the matching archive from the latest GitHub Release.
- Verifies the SHA-256 checksum against the published `SHA256SUMS` file.
- Verifies the cosign signature on `SHA256SUMS` if `cosign` is on `$PATH`.
- Refuses to run as root unless `ALLOW_ROOT=1` is set.
- Installs to `~/.local/bin/dots` by default (override with `INSTALL_DIR`).

For the strictest install, skip the pipe-to-sh and verify by hand — see
[RELEASING.md](./RELEASING.md#verifying-a-downloaded-release).

### Nix flake

```sh
nix run github:sanurb/.dotfiles
```

For the full home-manager-managed environment, follow the flake usage in
`flake.nix` — `dots` is one component of that environment, not the whole
thing.

## Verifying a release

Every release archive and the `SHA256SUMS` file are signed with cosign using
keyless OIDC (no managed keys; the certificate identity is the GitHub Actions
workflow). The verification recipe is in
[RELEASING.md](./RELEASING.md#verifying-a-downloaded-release) and embedded in
each GitHub Release's notes.

## Development

The dev shell is fully declarative — entering the directory provisions every
runtime, language server, and release tool:

```sh
direnv allow   # one-time
# everything below works inside the shell that direnv just activated
moon run cli:check
moon run cli:test
moon run dotfiles:deploy
```

See `DESIGN.md` for the architecture and `devenv.nix` for the declared
toolchain.

[tap]: https://github.com/sanurb/homebrew-tap
