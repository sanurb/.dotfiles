#!/bin/bash
# modules/scripts/deploy.sh — invoked by `moon run modules:deploy`.
#
# We live in a script file rather than inline in moon.yml's bash -c
# block because moon's task-arg interpolator rewrites every "$VAR"
# pattern at task-spawn time — including bash-local variables. An
# inline body containing `sys=...; if [ -z "$sys" ]` becomes
# `sys=...; if [ -z "" ]` after moon substitutes the (undefined-in-
# moon-env) `$sys` with empty. Putting the body in its own file
# moves it out of moon's interpolation surface; moon only sees the
# script path + its arguments.
#
# Argv contract: $1 is the nix system identifier (e.g.,
# "aarch64-darwin"). When called via `dots apply`, dots passes the
# pre-computed value through DOTS_NIX_SYSTEM env which moon then
# interpolates into "$1". Direct `moon run modules:deploy` with no
# DOTS_NIX_SYSTEM set falls back to `nix eval`.
#
# set -eu: surface unset vars and failed commands. pipefail
# intentionally not set — nh's stdout may legitimately produce
# broken pipes.

set -eu

sys="${1:-}"

if [ -z "$sys" ]; then
  sys="$(nix eval --raw --impure --expr 'builtins.currentSystem' 2>/dev/null || true)"
fi

if [ -z "$sys" ]; then
  echo "modules:deploy: cannot resolve nix system identifier" >&2
  echo "  what: DOTS_NIX_SYSTEM is unset and 'nix eval' produced no output" >&2
  echo "  why:  invoked outside 'dots apply' and nix is not on this PATH" >&2
  echo "  next: run 'dots apply' (sets DOTS_NIX_SYSTEM directly) or activate the dev shell" >&2
  exit 1
fi

exec nh home switch -c "$sys" . -- \
  --impure --accept-flake-config
