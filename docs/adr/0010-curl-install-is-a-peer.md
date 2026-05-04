# Curl-install is a peer of Homebrew and `nix run`

The `curl … install.sh | sh` path is a first-class install path, not
a fallback. The release pipeline ships a stable raw-binary asset
(`dots-{darwin,linux}-{amd64,arm64}` at `/releases/latest/download/`)
signed with cosign keyless OIDC; the fetcher verifies SHA-256
unconditionally and the cosign signature when `cosign` is on `$PATH`.
Homebrew tap auto-bump lag is the friction this path exists to
bypass: a fresh release is reachable by curl as soon as the GitHub
Release publishes.

## Consequences

The release pipeline's two archive entries (versioned `tar.gz` and
raw stable-name binary) plus matching cosign signatures are
load-bearing for this path; collapsing them back to a single
versioned archive would break it.
