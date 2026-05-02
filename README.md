# dots

Interactive frontend for a Nix-managed personal environment. The Go binary is
the user-facing layer; Moon + Nix + Home Manager are the deterministic
backend. `dots` never mutates `~/` directly — it scans, prompts, and delegates
to `moon run dotfiles:<task>`.

## Installation

`dots` is two components: a TUI binary that writes a profile, and a Nix
flake + Home Manager modules that realize the profile into your `$HOME`.
The TUI runs anywhere; realization needs Nix and a clone of this repo.
That asymmetry produces three install paths — pick the one that matches
how far you want to go.

### Path 1 — Try the TUI (no Nix required)

```sh
brew install sanurb/tap/dots
dots install
```

Walks you through the wizard and writes the profile to disk. Realization
(`dots deploy`) needs Nix and the workspace; without those, the TUI is
still a valid way to see what a profile looks like. Continue with Path 2
or 3 when you're ready to apply it.

### Path 2 — Full install (recommended)

```sh
brew install sanurb/tap/dots
dots install                                # configure profile via TUI
# Install Nix if you don't have it: https://determinate.systems/nix-installer
git clone https://github.com/sanurb/.dotfiles ~/.dotfiles
cd ~/.dotfiles
dots deploy                                 # realize the profile
```

The Homebrew binary handles the interactive bits; the clone + Nix handle
realization. Re-run `dots deploy` after editing your profile or pulling
upstream changes.

### Path 3 — Nix-native (no Homebrew)

```sh
# Requires Nix installed already.
nix run github:sanurb/.dotfiles -- install
nix run github:sanurb/.dotfiles -- deploy
```

Builds `dots` from source against the Nix store. Useful on hosts without
Homebrew or where you want one fewer package manager in the loop.

### Prerequisites

| Requirement | Path 1 | Path 2 | Path 3 |
|-------------|:------:|:------:|:------:|
| Homebrew    | ✓      | ✓      |        |
| Nix         |        | ✓      | ✓      |
| Repo clone  |        | ✓      |        |

Nix installer: https://determinate.systems/nix-installer.

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
