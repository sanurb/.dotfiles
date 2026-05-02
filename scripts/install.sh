#!/usr/bin/env bash
# install.sh — fetch the latest dots release, verify, install.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/sanurb/.dotfiles/main/scripts/install.sh | bash
#
# Pipe to `bash`, not `sh`. POSIX `sh` (dash on Debian/Ubuntu, /bin/sh on
# Alpine) does not support [[ ]], arrays, or `${!var}` indirect expansion;
# the shebang is ignored when the script runs from stdin.
#
# Environment:
#   INSTALL_DIR   target dir for the binary (default: ~/.local/bin)
#   VERSION       release tag to install (default: latest)
#   ALLOW_ROOT    set to 1 to allow running as root (default: refuse)
#   GITHUB        override the GitHub host (default: https://github.com)
#
# Verification:
#   The script always verifies the SHA-256 of the downloaded archive
#   against the published SHA256SUMS file. If `cosign` is on PATH, the
#   SHA256SUMS file's keyless OIDC signature is also verified. For the
#   strictest install, fetch + verify by hand following RELEASING.md.

set -euo pipefail

GITHUB="${GITHUB:-https://github.com}"
REPO="sanurb/.dotfiles"
BINARY="dots"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"
ALLOW_ROOT="${ALLOW_ROOT:-0}"

# ANSI colors when stdout is a TTY. When piped to a non-TTY (CI logs,
# `tee install.log`), strings stay plain.
Color_Off=''; Red=''; Green=''; Yellow=''; Dim=''; Bold_White=''; Bold_Green=''
if [[ -t 1 ]]; then
  Color_Off='\033[0m'
  Red='\033[0;31m'
  Green='\033[0;32m'
  Yellow='\033[0;33m'
  Dim='\033[0;2m'
  Bold_White='\033[1m'
  Bold_Green='\033[1;32m'
fi

error()     { echo -e "${Red}error${Color_Off}: $*" >&2; exit 1; }
warn()      { echo -e "${Yellow}warning${Color_Off}: $*" >&2; }
info()      { echo -e "${Dim}$* ${Color_Off}"; }
info_bold() { echo -e "${Bold_White}$* ${Color_Off}"; }
success()   { echo -e "${Green}$* ${Color_Off}"; }

tildify() {
  if [[ "$1" = "$HOME"/* ]]; then
    # Intentional literal "~" — this is a display string for the user,
    # not a path the shell will re-expand.
    # shellcheck disable=SC2088
    echo "~/${1#"$HOME"/}"
  else
    echo "$1"
  fi
}

# --- Pre-flight ----------------------------------------------------------

if [[ "$(id -u)" -eq 0 && "$ALLOW_ROOT" != "1" ]]; then
  error "refusing to run as root. Re-run as your user, or set ALLOW_ROOT=1 to override."
fi

command -v curl >/dev/null || error "curl is required to install dots"
command -v tar  >/dev/null || error "tar is required to install dots"

# Pick a SHA-256 implementation. macOS 13+ ships its own `sha256sum`
# without GNU's `-c` flag, so we don't use `-c` anywhere; we just compute
# the digest and string-compare against SHA256SUMS.
sha256_of() {
  if   command -v shasum    >/dev/null; then shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null; then sha256sum   "$1" | awk '{print $1}'
  elif command -v openssl   >/dev/null; then openssl dgst -sha256 "$1" | awk '{print $NF}'
  else error "no SHA-256 tool found (need shasum, sha256sum, or openssl)"
  fi
}

# --- Platform detection --------------------------------------------------

platform=$(uname -ms)
case "$platform" in
  'Darwin x86_64')                 target=darwin_amd64 ;;
  'Darwin arm64')                  target=darwin_arm64 ;;
  'Linux x86_64')                  target=linux_amd64  ;;
  'Linux aarch64' | 'Linux arm64') target=linux_arm64  ;;
  *) error "unsupported platform: $platform" ;;
esac

# Rosetta 2: a darwin/amd64 shell on Apple Silicon would download an
# x86_64 binary that runs through translation. Prefer the native build.
if [[ "$target" = "darwin_amd64" ]] \
  && [[ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = "1" ]]; then
  target=darwin_arm64
  info "Your shell is running in Rosetta 2 — downloading native dots for $target instead."
fi

# --- Resolve version -----------------------------------------------------

if [[ "$VERSION" = "latest" ]]; then
  info "resolving latest release"
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [[ -n "$VERSION" ]] || error "could not resolve latest release tag"
fi

version_no_v="${VERSION#v}"
archive="${BINARY}_${version_no_v}_${target}.tar.gz"
base_url="$GITHUB/$REPO/releases/download/$VERSION"

# --- Download ------------------------------------------------------------

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "downloading $archive ($VERSION)"
curl --fail --location --progress-bar --output "$tmp/$archive"   "$base_url/$archive" \
  || error "failed to download $base_url/$archive"
curl --fail --location --silent       --output "$tmp/SHA256SUMS" "$base_url/SHA256SUMS" \
  || error "failed to download SHA256SUMS"

# --- Verify --------------------------------------------------------------

info "verifying SHA-256"
expected=$(awk -v f="$archive" '$2 == f || $2 == "*"f { print $1; exit }' "$tmp/SHA256SUMS")
[[ -n "$expected" ]] || error "no checksum entry for $archive in SHA256SUMS"
actual=$(sha256_of "$tmp/$archive")
if [[ "$expected" != "$actual" ]]; then
  error "checksum mismatch for $archive
    expected: $expected
    actual:   $actual"
fi

if command -v cosign >/dev/null; then
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
tar -xzf "$tmp/$archive" -C "$tmp" || error "failed to extract $archive"
install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY" \
  || error "failed to install binary to $(tildify "$INSTALL_DIR")"

exe="$INSTALL_DIR/$BINARY"
success "$BINARY $VERSION was installed successfully to ${Bold_Green}$(tildify "$exe")${Color_Off}"

# Already on PATH? We're done.
if command -v "$BINARY" >/dev/null && [[ "$(command -v "$BINARY")" = "$exe" ]]; then
  echo
  info "Run '$BINARY --help' to get started"
  exit 0
fi

# --- PATH integration ----------------------------------------------------
#
# Append the export line to the user's shell rc, idempotently. Bun's
# installer appends unconditionally; we grep for a marker first so a
# reinstall doesn't pile up duplicates.

echo

# Render INSTALL_DIR for embedding inside a double-quoted string. Inside
# user $HOME, prefer "$HOME/...": it travels across machines if they ever
# move home dirs. Outside, use the absolute path. Either way, no extra
# layer of literal quotes — the template puts those.
if [[ "$INSTALL_DIR" = "$HOME"/* ]]; then
  install_dir_lit="\$HOME/${INSTALL_DIR#"$HOME"/}"
else
  install_dir_lit="$INSTALL_DIR"
fi

refresh_command=''
user_shell=$(basename "${SHELL:-sh}")

append_if_missing() {
  # $1 = config file, $2... = lines to append
  local cfg="$1"; shift
  local marker="# dots"
  mkdir -p "$(dirname "$cfg")"
  if [[ -f "$cfg" ]] && grep -qF "$marker" "$cfg"; then
    info "$(tildify "$cfg") already has a dots section — leaving it alone"
    return 0
  fi
  {
    echo
    echo "$marker"
    for line in "$@"; do echo "$line"; done
  } >> "$cfg"
  info "Added $(tildify "$INSTALL_DIR") to \$PATH in $(tildify "$cfg")"
}

case "$user_shell" in
  fish)
    cmd="fish_add_path \"$install_dir_lit\""
    cfg="$HOME/.config/fish/config.fish"
    if [[ -w "$cfg" ]] || [[ ! -e "$cfg" ]]; then
      append_if_missing "$cfg" "$cmd"
      refresh_command="source $(tildify "$cfg")"
    else
      echo "Manually add to $(tildify "$cfg"):"
      info_bold "  $cmd"
    fi
    ;;
  zsh)
    cmd="export PATH=\"$install_dir_lit:\$PATH\""
    cfg="$HOME/.zshrc"
    if [[ -w "$cfg" ]] || [[ ! -e "$cfg" ]]; then
      append_if_missing "$cfg" "$cmd"
      refresh_command="exec ${SHELL:-zsh}"
    else
      echo "Manually add to $(tildify "$cfg"):"
      info_bold "  $cmd"
    fi
    ;;
  bash)
    cmd="export PATH=\"$install_dir_lit:\$PATH\""
    bash_configs=("$HOME/.bashrc" "$HOME/.bash_profile")
    [[ -n "${XDG_CONFIG_HOME:-}" ]] && bash_configs+=("$XDG_CONFIG_HOME/bash/bashrc")
    set_manually=true
    for cfg in "${bash_configs[@]}"; do
      if [[ -w "$cfg" ]] || { [[ ! -e "$cfg" ]] && [[ -w "$(dirname "$cfg")" ]]; }; then
        append_if_missing "$cfg" "$cmd"
        refresh_command="source $(tildify "$cfg")"
        set_manually=false
        break
      fi
    done
    if $set_manually; then
      echo "Manually add to a bash rc:"
      info_bold "  $cmd"
    fi
    ;;
  nu | nushell)
    cmd="\$env.PATH = (\$env.PATH | split row (char esep) | prepend \"$INSTALL_DIR\")"
    cfg="$HOME/.config/nushell/env.nu"
    if [[ -w "$cfg" ]] || [[ ! -e "$cfg" ]]; then
      append_if_missing "$cfg" "$cmd"
      refresh_command="exec ${SHELL:-nu}"
    else
      echo "Manually add to $(tildify "$cfg"):"
      info_bold "  $cmd"
    fi
    ;;
  *)
    echo "Manually add to your shell rc:"
    info_bold "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo
info "To get started, run:"
echo
[[ -n "$refresh_command" ]] && info_bold "  $refresh_command"
info_bold "  $BINARY --help"
