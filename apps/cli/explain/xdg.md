# dots explain: xdg

dots reads and writes a small set of files under XDG-conformant paths,
with DOTS_*-prefixed overrides for power users and tests.

Resolution order, highest precedence first:

Config (read)

1. --config FILE (per-invocation)
2. $DOTS_CONFIG_HOME/dots/config.toml
3. $XDG_CONFIG_HOME/dots/config.toml
4. ~/.config/dots/config.toml

State (read/write — the procedural receipt of the last apply)

1. $DOTS_STATE_HOME/dots/applied.toml
2. $XDG_STATE_HOME/dots/applied.toml
3. ~/.local/state/dots/applied.toml

Cache (read/write — derivations, fetch caches, plan archives)

1. $DOTS_CACHE_HOME/dots/...
2. $XDG_CACHE_HOME/dots/...
3. ~/.cache/dots/...

The workspace's own `.dots-state.toml` is NOT under XDG. It is the
declarative _input_ that home.nix reads via builtins.fromTOML, and it
must travel with the repo so multiple machines pinning the same commit
realize the same persona. Confusing the two is a common first mistake;
they have deliberately different names.

Other env vars dots honors:
NO_COLOR disables ANSI in human output (also: --no-color).
CI treated as --non-interactive (no prompts).
MOON_WORKSPACE_ROOT
short-circuits the workspace-root walk; set by Moon's
task runner.
