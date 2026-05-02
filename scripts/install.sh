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

# Color output when stdout is a TTY. When piped (`curl | sh`), stdin is the
# pipe but stdout is still the user's terminal, so colors stay readable.
if [ -t 1 ] && command -v tput >/dev/null 2>&1 && [ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]; then
  C_RESET=$(tput sgr0)
  C_BOLD=$(tput bold)
  C_DIM=$(tput dim)
  C_RED=$(tput setaf 1)
  C_GREEN=$(tput setaf 2)
  C_YELLOW=$(tput setaf 3)
  C_BLUE=$(tput setaf 4)
else
  C_RESET=""; C_BOLD=""; C_DIM=""; C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""
fi

err()  { printf '%sinstall.sh:%s %s%s\n' "$C_RED$C_BOLD" "$C_RESET" "$*" "$C_RESET" >&2; exit 1; }
info() { printf '%sinstall.sh:%s %s\n' "$C_BLUE$C_BOLD" "$C_RESET" "$*"; }
warn() { printf '%sinstall.sh:%s %s\n' "$C_YELLOW$C_BOLD" "$C_RESET" "$*"; }
ok()   { printf '%sinstall.sh:%s %s\n' "$C_GREEN$C_BOLD" "$C_RESET" "$*"; }

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
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *)             err "unsupported arch: $uname_arch" ;;
esac

# Required tools (just for download + extract; hash tool is selected below).
for cmd in curl tar; do
  command -v "$cmd" >/dev/null 2>&1 || err "missing required command: $cmd"
done

# Pick a SHA-256 implementation that exists and is portable. Earlier versions
# of this script piped into `<tool> -c <SUMS>`, but `-c` is a GNU extension;
# macOS ships its own `sha256sum` (since 13) that doesn't support it, so
# the `-c` path fails with "usage:". We just compute the digest ourselves
# and string-compare — works the same on every platform.
sha256_of() {
  # $1 = path → emits the lowercase hex digest on stdout
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$1" | awk '{print $NF}'
  else
    err "no SHA-256 tool found (need one of: shasum, sha256sum, openssl)"
  fi
}

# Resolve version.
if [ "$VERSION" = "latest" ]; then
  info "resolving latest release"
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || err "could not resolve latest release tag"
fi
# Strip leading 'v' for filename (goreleaser drops it).
version_no_v="${VERSION#v}"

archive="${BINARY}_${version_no_v}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$VERSION"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "downloading ${C_BOLD}${archive}${C_RESET} (${VERSION}, ${os}/${arch})"
curl -fsSL -o "$tmp/$archive"   "$base_url/$archive"
curl -fsSL -o "$tmp/SHA256SUMS" "$base_url/SHA256SUMS"

info "verifying SHA-256"
expected=$(awk -v f="$archive" '$2 == f || $2 == "*"f { print $1; exit }' "$tmp/SHA256SUMS")
[ -n "$expected" ] || err "no checksum entry for $archive in SHA256SUMS"
actual=$(sha256_of "$tmp/$archive")
if [ "$expected" != "$actual" ]; then
  err "checksum mismatch for $archive
  expected: $expected
  actual:   $actual"
fi

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
  warn "cosign not on PATH — skipping signature verification (SHA-256 still verified)"
fi

info "extracting"
tar -xzf "$tmp/$archive" -C "$tmp"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"

ok "installed ${C_BOLD}${BINARY} ${VERSION}${C_RESET} → ${INSTALL_DIR}/${BINARY}"

# PATH guidance with shell-specific rc-file hints, à la bun's installer.
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    warn "$INSTALL_DIR is not on \$PATH"
    user_shell=$(basename "${SHELL:-sh}")
    case "$user_shell" in
      bash)    rc='~/.bashrc';      export_line="export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
      zsh)     rc='~/.zshrc';       export_line="export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
      fish)    rc='~/.config/fish/config.fish'; export_line="fish_add_path $INSTALL_DIR" ;;
      nu*)     rc='~/.config/nushell/env.nu';   export_line="\$env.PATH = (\$env.PATH | split row (char esep) | prepend \"$INSTALL_DIR\")" ;;
      *)       rc='your shell rc';  export_line="export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
    esac
    printf '       Add to %s%s%s:\n         %s%s%s\n' "$C_BOLD" "$rc" "$C_RESET" "$C_DIM" "$export_line" "$C_RESET"
    ;;
esac

"$INSTALL_DIR/$BINARY" --version
