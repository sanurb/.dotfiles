# Activation is dots-direct; moon never orchestrates Nix

`dots apply` execs `nh home switch` directly from the Go process.
Moon's role is the Go build/test/lint DAG and a thin `modules:check`
wrapper around `nix flake check`; it never sits between `dots` and
`nh`. A prior design routed activation through `moon run
modules:deploy`, and moon's argv interpolator silently rewrote
bash-local `$VAR` references in the task body — an entire class of
bug only avoidable by not putting moon in the activation path.

## Consequences

- Reintroducing a `modules:deploy` task, or putting multi-line bash
  bodies in `modules/moon.yml`, recreates the interpolation footgun.
- If a moon-driven shell script is ever genuinely needed, the body
  lives in a `.sh` file and moon sees only the script path.
- All Nix subprocess invocations belong in the Go CLI, not in moon
  tasks.
