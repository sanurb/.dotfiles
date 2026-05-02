# Releasing `dots`

`dots` is released via [GoReleaser](https://goreleaser.com/) on every git tag
that matches `v*`. The pipeline produces signed, multi-arch archives, an
SBOM, a Homebrew formula update, and a GitHub Release.

## What a release ships (and what it does not)

`dots` has two components, and only one is in the release artifact:

- **Released:** the `dots` binary — the TUI plus the realization-adjacent
  subcommands. Every subcommand is compiled into the same binary.
  Workspace-required subcommands (`deploy`, `doctor`, `sync`, `scan`,
  `backup`) ship in the binary but exit with code 2 and an actionable
  message when invoked outside a workspace; the binary alone cannot
  realize a profile.
- **Not released:** the Nix flake, the Home Manager modules, the
  realization layer. These are obtained by cloning the repo or by
  invoking `nix run github:sanurb/.dotfiles -- <subcommand>`. They are
  versioned by git, not by SemVer tags.

The Homebrew formula and the GitHub Release archives carry only the
binary. A user on the `try-the-TUI` install path (`brew install …`)
gets a working `dots install` and `dots version`; running `dots deploy`
from that path prints the workspace-required message and exits 2 —
this is by design.

## Versioning

SemVer (`vMAJOR.MINOR.PATCH`).

- **Major** bump: breaking change to the on-disk state schema
  (`.dots-state.toml` keys / value sets) consumed by `home.nix`, or
  removal of a subcommand.
- **Minor** bump: new subcommand, new state field with a safe default,
  new TUI step.
- **Patch** bump: bug fixes, dependency bumps, doc-only changes.

The flake and the binary share the tag space — a `v1.x` binary expects
the `v1.x` flake at the workspace root. Mixing major versions across
the two is unsupported.

## One-time prerequisites

Before the first tagged release, the following must exist:

1. **Repository:** `github.com/sanurb/.dotfiles` (this repo).
2. **Homebrew tap repo:** `github.com/sanurb/homebrew-tap`. Create it as an
   empty public repo — GoReleaser populates `Formula/dots.rb` on each
   release.
3. **Repo secret `HOMEBREW_TAP_TOKEN`** on `sanurb/.dotfiles`. A
   fine-grained Personal Access Token scoped to `sanurb/homebrew-tap` with
   **Contents: read & write**. The default `GITHUB_TOKEN` cannot push to a
   different repo, which is why the tap requires this PAT.

No cosign keys to manage — signing uses keyless OIDC. The release workflow
declares `id-token: write`, which is sufficient.

## Cutting a release

```sh
# 1. Make sure main is green and you are on a clean tree.
git switch main
git pull --ff-only
git status            # must be clean

# 2. (Optional) Validate the release config locally.
goreleaser check
goreleaser release --snapshot --clean --skip=publish,sign

# 3. Tag and push. The tag triggers .github/workflows/release.yml.
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0

# 4. Watch the run.
gh run watch
```

A successful run produces:

- `dots_<version>_{darwin,linux}_{amd64,arm64}.tar.gz` × 4
- `SHA256SUMS` + `SHA256SUMS.sig` + `SHA256SUMS.pem`
- One `.sig` and `.pem` per archive
- One `.sbom.json` (CycloneDX) per archive
- A Homebrew formula commit on `sanurb/homebrew-tap`
- A GitHub Release with all of the above as assets

## Pre-release checklist

- [ ] `moon run cli:check` is green on `main`.
- [ ] `moon run cli:test` is green on `main`.
- [ ] `goreleaser check` passes.
- [ ] The tag follows SemVer (`vMAJOR.MINOR.PATCH`).
- [ ] Pre-release tags use `-rc.N`, `-beta.N`, etc. — GoReleaser auto-detects
      these and marks the GitHub Release as a pre-release.
- [ ] If anything in the install script changed, the Homebrew formula change,
      or signed-blob verification recipe changed, `README.md` reflects it.

## Verifying a downloaded release

```sh
VERSION=v0.1.0
OS=darwin                 # or linux
ARCH=arm64                # or amd64
BASE="https://github.com/sanurb/.dotfiles/releases/download/$VERSION"

curl -fsSLO "$BASE/SHA256SUMS"
curl -fsSLO "$BASE/SHA256SUMS.sig"
curl -fsSLO "$BASE/SHA256SUMS.pem"
curl -fsSLO "$BASE/dots_${VERSION#v}_${OS}_${ARCH}.tar.gz"

# 1. Verify the checksum file's signature.
cosign verify-blob \
  --certificate-identity-regexp "https://github.com/sanurb/.dotfiles" \
  --certificate-oidc-issuer     "https://token.actions.githubusercontent.com" \
  --certificate                 SHA256SUMS.pem \
  --signature                   SHA256SUMS.sig \
  SHA256SUMS

# 2. Verify the archive matches its checksum.
grep "dots_${VERSION#v}_${OS}_${ARCH}.tar.gz" SHA256SUMS | shasum -a 256 -c -
```

## Rolling back a bad release

GoReleaser releases are immutable assets, but a bad tag can be defused:

1. **Mark the GitHub Release as a draft** (or delete it):
   ```sh
   gh release edit v0.1.0 --draft
   # or
   gh release delete v0.1.0 --yes
   ```
2. **Delete the tag** locally and on the remote:
   ```sh
   git tag -d v0.1.0
   git push origin :refs/tags/v0.1.0
   ```
3. **Revert the Homebrew formula bump** on `sanurb/homebrew-tap`:
   ```sh
   gh repo clone sanurb/homebrew-tap /tmp/tap && cd /tmp/tap
   git revert <commit-of-dots-bump>
   git push origin main
   ```
4. Cut a fresh tag with the next patch version (e.g., `v0.1.1`). Do not
   re-use the deleted tag — Homebrew, install-script users, and the SBOM
   index would still see the old artifact in any cached state.

## Out of scope (today)

- macOS code-signing / notarization (Apple Developer cert).
- Linux package formats (`.deb`, `.rpm`, AUR).
- A custom domain for the install script.
- Cachix / Attic binary cache for the Nix flake.

These are deliberate non-goals for v1; revisit when adoption justifies the
ceremony.
