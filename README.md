# dots

Interactive frontend for a Nix-managed personal environment. The Go binary is
the user-facing layer; Moon + Nix + Home Manager are the deterministic
backend. `dots` never mutates `~/` directly — it scans, prompts, and delegates
to `moon run modules:<task>`.

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

Outside a workspace, `dots install` runs in standalone mode: it walks
the wizard and writes `~/.config/dots/selection.toml`, then prints next
steps. Realization (`dots deploy`) needs Nix and the workspace; without
those, the TUI is still a valid way to capture a profile. Continue with
Path 2 or 3 when you're ready to apply it.

### Path 2 — Full install (recommended)

```sh
brew install sanurb/tap/dots
dots install                                # captures profile via TUI
# Install Nix if you don't have it: https://determinate.systems/nix-installer
git clone https://github.com/sanurb/.dotfiles ~/.dotfiles
cd ~/.dotfiles
nix develop -c dots deploy                  # realize the profile
```

`dots deploy` shells out to Moon, which is provisioned by the dev
shell. The `nix develop -c` prefix enters that shell for the single
command. For the canonical UX (auto-activated shell on `cd` into the
repo), install [direnv][] (`brew install direnv`, hook it into your
shell, then `direnv allow`); after that, `dots deploy` works without
the prefix.

Re-run `dots deploy` after editing your profile or pulling upstream
changes.

### Path 3 — Curl-install (no Homebrew)

```sh
curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh | sh
```

Drops the binary at `~/.local/bin/dots` (override with `INSTALL_DIR=`).
The fetcher is POSIX `sh`, always verifies SHA-256 against the
published `SHA256SUMS`, and verifies the cosign signature when
`cosign` is on `$PATH`. Manual fetch + verify is documented in
[RELEASING.md](./RELEASING.md#verifying-a-downloaded-release).

This delivers only the binary. Realization is the same as Path 2 —
clone, enter the dev shell, `dots deploy`.

### Prerequisites

| Requirement | Path 1 | Path 2 | Path 3 |
|-------------|:------:|:------:|:------:|
| Homebrew    | ✓      | ✓      |        |
| Nix         |        | ✓      | ✓      |
| Repo clone  |        | ✓      | ✓      |

Nix is required for `dots deploy` regardless of how the binary
arrived. Nix installer: https://determinate.systems/nix-installer.

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
moon run modules:deploy
```

See `DESIGN.md` for the architecture and `devenv.nix` for the declared
toolchain.

[direnv]: https://direnv.net/
[tap]: https://github.com/sanurb/homebrew-tap
