#!/usr/bin/env sh
# install.sh — fetch the latest dots release, verify, install.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh | sh
#
# POSIX-compatible: the shebang is ignored when the script runs from
# stdin, so this script avoids bashisms ([[ ]], arrays, indirect
# expansion, `echo -e`) on purpose. It runs identically under sh, bash,
# zsh, dash, ash, busybox sh.
#
# Environment:
#   INSTALL_DIR   target dir for the binary (default: ~/.local/bin)
#   VERSION       release tag to install (default: latest)
#   ALLOW_ROOT    set to 1 to allow running as root (default: refuse)
#   GITHUB        override the GitHub host (default: https://github.com)
#
# Verification:
#   The script always verifies the SHA-256 of the downloaded binary
#   against the published SHA256SUMS file. If `cosign` is on PATH, the
#   SHA256SUMS file's keyless OIDC signature is also verified. For the
#   strictest install, fetch + verify by hand following RELEASING.md.

set -eu

GITHUB="${GITHUB:-https://github.com}"
REPO="sanurb/.dotfiles"
BINARY="dots"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"
ALLOW_ROOT="${ALLOW_ROOT:-0}"

# ANSI colors when stdout is a TTY. When piped to a non-TTY (CI logs,
# `tee install.log`), strings stay plain. Built with literal escape
# bytes via printf so we never depend on `echo -e` (POSIX `echo` does
# not support `-e`; bash-in-POSIX-mode prints it as text).
esc=$(printf '\033')
if [ -t 1 ]; then
  Color_Off="${esc}[0m"
  Red="${esc}[0;31m"
  Green="${esc}[0;32m"
  Yellow="${esc}[0;33m"
  Dim="${esc}[0;2m"
  Bold_White="${esc}[1m"
  Bold_Green="${esc}[1;32m"
else
  Color_Off=''; Red=''; Green=''; Yellow=''; Dim=''; Bold_White=''; Bold_Green=''
fi

error()     { printf '%serror%s: %s\n' "$Red"        "$Color_Off" "$*" >&2; exit 1; }
warn()      { printf '%swarning%s: %s\n' "$Yellow"   "$Color_Off" "$*" >&2; }
info()      { printf '%s%s%s\n' "$Dim"        "$*" "$Color_Off"; }
info_bold() { printf '%s%s%s\n' "$Bold_White" "$*" "$Color_Off"; }
success()   { printf '%s%s%s\n' "$Green"      "$*" "$Color_Off"; }

tildify() {
  case "$1" in
    "$HOME"/*)
      # Intentional literal "~" — this is a display string for the
      # user, not a path the shell will re-expand.
      # shellcheck disable=SC2088
      printf '~/%s\n' "${1#"$HOME"/}"
      ;;
    *)
      printf '%s\n' "$1"
      ;;
  esac
}

# --- Pre-flight ----------------------------------------------------------

if [ "$(id -u)" -eq 0 ] && [ "$ALLOW_ROOT" != "1" ]; then
  error "refusing to run as root. Re-run as your user, or set ALLOW_ROOT=1 to override."
fi

command -v curl >/dev/null 2>&1 || error "curl is required to install dots"

# Pick a SHA-256 implementation. macOS 13+ ships its own `sha256sum`
# without GNU's `-c` flag, so we don't use `-c` anywhere; we just
# compute the digest and string-compare against SHA256SUMS.
sha256_of() {
  if   command -v shasum    >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then sha256sum   "$1" | awk '{print $1}'
  elif command -v openssl   >/dev/null 2>&1; then openssl dgst -sha256 "$1" | awk '{print $NF}'
  else error "no SHA-256 tool found (need shasum, sha256sum, or openssl)"
  fi
}

# --- Platform detection --------------------------------------------------

platform=$(uname -ms)
case "$platform" in
  'Darwin x86_64')                 os=darwin; arch=amd64 ;;
  'Darwin arm64')                  os=darwin; arch=arm64 ;;
  'Linux x86_64')                  os=linux;  arch=amd64 ;;
  'Linux aarch64' | 'Linux arm64') os=linux;  arch=arm64 ;;
  *) error "unsupported platform: $platform — dots ships for macOS and Linux only" ;;
esac

# Rosetta 2: a darwin/amd64 shell on Apple Silicon would download an
# x86_64 binary that runs through translation. Prefer the native build.
if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ] \
  && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = "1" ]; then
  arch=arm64
  info "Your shell is running in Rosetta 2 — downloading native dots for $os-$arch instead."
fi

# --- Resolve version -----------------------------------------------------

if [ "$VERSION" = "latest" ]; then
  info "resolving latest release"
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || error "could not resolve latest release tag"
fi

asset="${BINARY}-${os}-${arch}"
base_url="$GITHUB/$REPO/releases/download/$VERSION"

# --- Download ------------------------------------------------------------

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "downloading $asset ($VERSION)"
curl --fail --location --progress-bar --output "$tmp/$asset"     "$base_url/$asset" \
  || error "failed to download $base_url/$asset"
curl --fail --location --silent       --output "$tmp/SHA256SUMS" "$base_url/SHA256SUMS" \
  || error "failed to download SHA256SUMS"

# --- Verify --------------------------------------------------------------

info "verifying SHA-256"
expected=$(awk -v f="$asset" '$2 == f || $2 == "*"f { print $1; exit }' "$tmp/SHA256SUMS")
[ -n "$expected" ] || error "no checksum entry for $asset in SHA256SUMS"
actual=$(sha256_of "$tmp/$asset")
if [ "$expected" != "$actual" ]; then
  error "checksum mismatch for $asset (expected $expected, got $actual)"
fi

if command -v cosign >/dev/null 2>&1; then
  info "verifying cosign signature"
  curl --fail --location --silent --output "$tmp/SHA256SUMS.sig" "$base_url/SHA256SUMS.sig"
  curl --fail --location --silent --output "$tmp/SHA256SUMS.pem" "$base_url/SHA256SUMS.pem"
  cosign verify-blob \
    --certificate-identity-regexp "https://github.com/$REPO" \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    --certificate "$tmp/SHA256SUMS.pem" \
    --signature   "$tmp/SHA256SUMS.sig" \
    "$tmp/SHA256SUMS" >/dev/null 2>&1 \
      || error "cosign signature verification failed"
else
  warn "cosign not on PATH — skipping signature verification (SHA-256 still verified)"
fi

# --- Install -------------------------------------------------------------

mkdir -p "$INSTALL_DIR" || error "failed to create install directory $(tildify "$INSTALL_DIR")"
install -m 0755 "$tmp/$asset" "$INSTALL_DIR/$BINARY" \
  || error "failed to install binary to $(tildify "$INSTALL_DIR")"

exe="$INSTALL_DIR/$BINARY"
success "$BINARY $VERSION was installed successfully to ${Bold_Green}$(tildify "$exe")${Color_Off}"

# Already on PATH and resolves to our just-installed binary? We're done.
if command -v "$BINARY" >/dev/null 2>&1 \
  && [ "$(command -v "$BINARY")" = "$exe" ]; then
  printf '\n'
  info "Run '$BINARY --help' to get started"
  exit 0
fi

# --- PATH integration ----------------------------------------------------
#
# Append the export line to the user's shell rc, idempotently. Bun's
# installer appends unconditionally; we grep for a marker first so a
# reinstall doesn't pile up duplicates.

printf '\n'

# Render INSTALL_DIR for embedding inside a double-quoted export. Inside
# user $HOME, prefer "$HOME/...": survives across moved home dirs.
case "$INSTALL_DIR" in
  "$HOME"/*) install_dir_lit="\$HOME/${INSTALL_DIR#"$HOME"/}" ;;
  *)         install_dir_lit="$INSTALL_DIR" ;;
esac

refresh_command=''
user_shell=$(basename "${SHELL:-sh}")

# Append a `# dots`-marked block to $1 if no such block exists yet.
# Remaining args are the lines of the block.
append_if_missing() {
  cfg="$1"; shift
  marker="# dots"
  mkdir -p "$(dirname "$cfg")"
  if [ -f "$cfg" ] && grep -qF "$marker" "$cfg"; then
    info "$(tildify "$cfg") already has a dots section — leaving it alone"
    return 0
  fi
  {
    printf '\n'
    printf '%s\n' "$marker"
    for line in "$@"; do printf '%s\n' "$line"; done
  } >> "$cfg"
  info "Added $(tildify "$INSTALL_DIR") to \$PATH in $(tildify "$cfg")"
}

case "$user_shell" in
  fish)
    cmd="fish_add_path \"$install_dir_lit\""
    cfg="$HOME/.config/fish/config.fish"
    if [ -w "$cfg" ] || [ ! -e "$cfg" ]; then
      append_if_missing "$cfg" "$cmd"
      refresh_command="source $(tildify "$cfg")"
    else
      printf 'Manually add to %s:\n' "$(tildify "$cfg")"
      info_bold "  $cmd"
    fi
    ;;
  zsh)
    cmd="export PATH=\"$install_dir_lit:\$PATH\""
    cfg="$HOME/.zshrc"
    if [ -w "$cfg" ] || [ ! -e "$cfg" ]; then
      append_if_missing "$cfg" "$cmd"
      refresh_command="exec ${SHELL:-zsh}"
    else
      printf 'Manually add to %s:\n' "$(tildify "$cfg")"
      info_bold "  $cmd"
    fi
    ;;
  bash)
    cmd="export PATH=\"$install_dir_lit:\$PATH\""
    set_manually=1
    for cfg in "$HOME/.bashrc" "$HOME/.bash_profile"; do
      if [ -w "$cfg" ] || { [ ! -e "$cfg" ] && [ -w "$(dirname "$cfg")" ]; }; then
        append_if_missing "$cfg" "$cmd"
        refresh_command="source $(tildify "$cfg")"
        set_manually=0
        break
      fi
    done
    if [ "$set_manually" = "1" ]; then
      printf 'Manually add to a bash rc:\n'
      info_bold "  $cmd"
    fi
    ;;
  nu | nushell)
    cmd="\$env.PATH = (\$env.PATH | split row (char esep) | prepend \"$INSTALL_DIR\")"
    cfg="$HOME/.config/nushell/env.nu"
    if [ -w "$cfg" ] || [ ! -e "$cfg" ]; then
      append_if_missing "$cfg" "$cmd"
      refresh_command="exec ${SHELL:-nu}"
    else
      printf 'Manually add to %s:\n' "$(tildify "$cfg")"
      info_bold "  $cmd"
    fi
    ;;
  *)
    printf 'Manually add to your shell rc:\n'
    info_bold "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

printf '\n'
info "To get started, run:"
printf '\n'
[ -n "$refresh_command" ] && info_bold "  $refresh_command"
info_bold "  $BINARY --help"
