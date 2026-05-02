#!/usr/bin/env sh
# install.sh — fetch the latest dots release, verify, install.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh | sh
#
# Environment:
#   INSTALL_DIR   target dir for the binary (default: ~/.local/bin)
#   VERSION       release tag to install (default: latest)
#   ALLOW_ROOT    set to 1 to allow running as root (default: refuse)
#
# Verification:
#   The script always verifies the SHA256 of the downloaded archive against
#   the published SHA256SUMS file. cosign signature verification is run
#   when `cosign` is found on PATH; otherwise it is skipped with a notice.
#   For the strictest install, fetch + verify by hand following RELEASING.md.

set -eu

REPO="sanurb/.dotfiles"
BINARY="dots"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"
ALLOW_ROOT="${ALLOW_ROOT:-0}"

err() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
info() { printf 'install.sh: %s\n' "$*"; }

if [ "$(id -u)" -eq 0 ] && [ "$ALLOW_ROOT" != "1" ]; then
  err "refusing to run as root. Re-run as your user, or set ALLOW_ROOT=1 if you really mean it."
fi

# Detect OS.
uname_os=$(uname -s)
case "$uname_os" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *)      err "unsupported OS: $uname_os" ;;
esac

# Detect arch.
uname_arch=$(uname -m)
case "$uname_arch" in
  x86_64|amd64)        arch=amd64 ;;
  arm64|aarch64)       arch=arm64 ;;
  *)                   err "unsupported arch: $uname_arch" ;;
esac

# Required tools.
for cmd in curl tar shasum; do
  command -v "$cmd" >/dev/null 2>&1 || err "missing required command: $cmd"
done
# `shasum` is BSD/macOS; Linux distros usually have `sha256sum`. Prefer the
# latter when available — its output format matches SHA256SUMS exactly.
if command -v sha256sum >/dev/null 2>&1; then
  SHA_CHECK="sha256sum -c"
else
  SHA_CHECK="shasum -a 256 -c"
fi

# Resolve version.
if [ "$VERSION" = "latest" ]; then
  info "resolving latest release..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || err "could not resolve latest release tag"
fi
# Strip leading 'v' for filename (goreleaser drops it).
version_no_v="${VERSION#v}"

archive="${BINARY}_${version_no_v}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$VERSION"
archive_url="$base_url/$archive"
sums_url="$base_url/SHA256SUMS"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "downloading $archive"
curl -fsSL -o "$tmp/$archive"     "$archive_url"
curl -fsSL -o "$tmp/SHA256SUMS"   "$sums_url"

info "verifying SHA256"
(cd "$tmp" && grep " $archive\$" SHA256SUMS | $SHA_CHECK >/dev/null) \
  || err "checksum verification failed for $archive"

if command -v cosign >/dev/null 2>&1; then
  info "verifying cosign signature"
  curl -fsSL -o "$tmp/SHA256SUMS.sig" "$base_url/SHA256SUMS.sig"
  curl -fsSL -o "$tmp/SHA256SUMS.pem" "$base_url/SHA256SUMS.pem"
  cosign verify-blob \
    --certificate-identity-regexp "https://github.com/$REPO" \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    --certificate "$tmp/SHA256SUMS.pem" \
    --signature   "$tmp/SHA256SUMS.sig" \
    "$tmp/SHA256SUMS" >/dev/null 2>&1 \
      || err "cosign signature verification failed"
else
  info "cosign not found on PATH — skipping signature verification (checksum still verified)"
fi

info "extracting"
tar -xzf "$tmp/$archive" -C "$tmp"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"

info "installed: $INSTALL_DIR/$BINARY"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) info "note: $INSTALL_DIR is not on \$PATH. Add it to your shell rc:"
     info "      export PATH=\"$INSTALL_DIR:\$PATH\""
     ;;
esac

"$INSTALL_DIR/$BINARY" --version
