# dots

Interactive frontend for a Nix-managed personal environment. The Go binary is
the user-facing layer; Moon + Nix + Home Manager are the deterministic
backend. `dots` never mutates `~/` directly — it scans, prompts, and delegates
to `moon run modules:<task>`.

## Installation

`dots` is two components: a TUI binary that writes a profile, and a Nix
flake + Home Manager modules that realize the profile into `$HOME`. The
TUI runs anywhere a Go binary runs; realization needs Nix and a clone of
this repo. There are three peer install paths — Homebrew, curl, and
`nix run` — pick the one that matches how you already get tools.

> **Supported platforms:** macOS and Linux. The realization layer
> depends on `nh` and Moon, neither of which has a Windows port; the
> TUI binary similarly does not ship for Windows. Windows users need
> WSL2 — both the TUI and `dots deploy` work inside it. (ADR-0011)

### Homebrew

```sh
brew install sanurb/tap/dots
```

Persistent binary at `$(brew --prefix)/bin/dots`. The tap
([sanurb/homebrew-tap][tap]) ships only `dots`; auto-bumps on every
release.

### Curl

```sh
curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh | sh
```

Persistent binary at `~/.local/bin/dots` (override with `INSTALL_DIR=`).
The fetcher is POSIX `sh`, downloads the stable raw-binary asset
(`dots-{darwin,linux}-{amd64,arm64}`), always verifies SHA-256, and
verifies the cosign keyless OIDC signature when `cosign` is on `$PATH`.

### `nix run`

```sh
nix run github:sanurb/.dotfiles -- install
```

Ephemeral execution — no persistent install. Requires Nix on the host
already; no clone required (Nix fetches the flake on demand).

### After install

```sh
dots install   # walk the wizard, capture ~/.config/dots/selection.toml
dots deploy    # realize the profile (workspace + Nix; offered if missing)
```

`dots install` writes the profile and (inside a workspace) hands off to
`dots deploy` for realization. `dots deploy` self-bootstraps Nix and the
workspace clone with explicit consent prompts — a fresh machine reaches
realization without manual prep, but every state-mutating step is
shown verbatim before it runs (ADR-0010).

For the canonical day-to-day UX, install [direnv][] so that entering
the cloned workspace activates the dev shell automatically; `dots
deploy` then runs without a `nix develop -c` prefix.

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
