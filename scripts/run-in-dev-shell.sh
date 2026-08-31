#!/usr/bin/env sh
# run-in-dev-shell.sh — execute repository tooling through the canonical Nix shell.
set -eu

if [ "$#" -eq 0 ]; then
  echo "run-in-dev-shell: expected a command" >&2
  exit 64
fi

workspace_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "run-in-dev-shell: not inside a Git workspace" >&2
  exit 1
}
cd "$workspace_root"

# Avoid nesting when direnv or an outer `nix develop` already activated the
# workspace. DEVENV_ROOT is set by Devenv and names this exact checkout.
if [ "${DEVENV_ROOT:-}" = "$workspace_root" ]; then
  exec "$@"
fi

if ! command -v nix >/dev/null 2>&1; then
  echo "run-in-dev-shell: Nix is required to run repository tooling" >&2
  echo "  fix: install Nix, then retry the command" >&2
  exit 127
fi

# flake.nix reads this ignored sentinel through the devenv-root input. Keep the
# write idempotent so merely running a Git hook does not invalidate direnv's
# cached profile on every invocation.
devenv_root_file="$workspace_root/.devenv-root"
if [ ! -f "$devenv_root_file" ] || [ "$(cat "$devenv_root_file")" != "$workspace_root" ]; then
  printf '%s\n' "$workspace_root" >"$devenv_root_file"
fi

exec nix develop \
  --impure \
  --accept-flake-config \
  --override-input devenv-root "file+file://$devenv_root_file" \
  -c "$@"
