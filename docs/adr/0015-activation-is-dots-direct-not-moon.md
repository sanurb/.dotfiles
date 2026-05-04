# ADR-0015: Activation is dots-direct; moon owns the dev DAG only

`dots apply`'s deploy path used to invoke `moon run modules:deploy`,
which ran a bash task that shelled out to `nh home switch`. Eight
patch releases (v0.4.0..v0.4.8) were spent making that chain work on
a non-direnv-activated shell. Each release fixed a different layer:
PATH augmentation for moon's task, PROTO_HOME for proto, an explicit
DOTS_NIX_SYSTEM env, an explicit env-block in moon.yml so moon
forwarded the var, externalizing the bash to a script file because
moon's argv interpolator rewrote bash-local `$VAR` references, and
finally surfacing nh's hidden activation logs. Each fix was correct
in isolation; the cumulative pattern told the real story.

The deploy path is not what moon is good at. Moon's design center is
"task graph with caching." The deploy path is a linear two-step
sequence — doctor, then activate — with `cache: false` on both nodes
and no parallelism available. Moon was earning none of its costs
back. What it *was* contributing was extra surface area: an argv
interpolator that rewrites every `$VAR`, an env-passthrough policy
that strips unknown keys by default, and a `$SHELL`-wrapping behavior
that ignores the `command:` field. Each of these surfaced as a
different bug between v0.4.0 and v0.4.7.

This ADR collapses the deploy path into Go. `dots apply`'s
`apply-profile` step calls `runDoctor` and `runHomeActivation`
in-process. `runHomeActivation` resolves nh against an augmented
PATH (the workspace's `.devenv/profile/bin` is prepended so nh is
reachable without direnv activation) and execs `nh home switch
--show-activation-logs -c <system> .` directly. The `<system>`
identifier is `plan.CurrentHost().NixIdent()` (one Go function, used
by both production and the e2e test mirror). No moon, no bash, no
argv interpolation, no env-passthrough surprise. Three layers
collapse to one.

A new `--no-preflight` flag on `dots apply` lets power users and the
e2e test harness skip the doctor step. Default behavior is unchanged
from prior releases: doctor runs, gates the activation, and a
failure exits with code 4 (PreFlight) before nh is invoked.

What stays in moon: every dev task that earns moon's caching or
graph value. `apps/cli/moon.yml` (build/test/check/doctor),
`.moon/tasks/tag-go.yml` (the Go DAG), `modules/moon.yml::check`
(`nix flake check` — caching matters, no argv interpolation issues
because it's a single literal command), `modules/moon.yml::update`
(`nix flake update`), and `root:fmt` (treefmt dispatch). ADR-0005's
"moon owns Go DAG, never Nix" reads more honestly now: `modules:deploy`
was the partial violation, and it's gone.

What's removed: the `modules:deploy` moon task, `modules/scripts/
deploy.sh` (the script we created in v0.4.7 to escape moon's
interpolator — no longer needed once moon is out of the deploy
path), and the orphan `modules/scripts/backup.sh` (its `dots
backup` Go counterpart was already the canonical surface; the
shell script was a mirror nobody invoked).

Trade-offs. Direct invocations of `moon run modules:deploy` no
longer work — but that path was never the documented UX (`dots
apply` is). ADR-0009's "TUI invokes deploy via subprocess" still
holds: the wizard re-execs `dots apply --yes`. ADR-0010's
self-bootstrap contract still holds: `dots apply` orchestrates the
plan and runs each step. ADR-0014's plan-as-artifact still holds:
the apply-profile step's `Command` field now displays the literal
`nh home switch ...` invocation, computed once via
`applyProfileCommand()` so plan rendering and runtime invocation
cannot drift.

Alternatives considered. Keep `modules:deploy` as a thin moon shim
that exec's `dots apply --yes`: rejected — symmetric indirection
without value. Keep the bash script and just remove the moon task:
rejected — the script existed only to escape moon's interpolator;
once moon is gone the script's reason to exist goes with it. Pin
moon to a version with cleaner interpolation semantics: not
available, and a tooling-pin doesn't address the architectural
mismatch.

Pre-flight gating opt-out. The `--no-preflight` flag is the explicit
escape hatch for two cases: e2e tests where doctor would fail in a
synthetic workspace, and power users running apply in a tight loop
who already know their toolchain is good. Default behavior preserves
the doctor-as-gate contract from ADR-0010. Users who want
post-failure visibility for the doctor checks themselves run
`dots doctor` directly.
