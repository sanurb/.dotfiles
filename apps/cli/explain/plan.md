# dots explain: plan

A Plan is the first-class artifact every state-changing dots verb computes
before it touches the system. Apply, deploy, rollback — none of them mutate
anything until a Plan has been produced, displayed, and (interactively or via
--yes) accepted.

The Plan is content-addressed: a SHA-256 over its schemaVersion, profile,
host fingerprint, and ordered step list. Two computations of the same Plan
on the same host therefore hash identically (the timestamp is excluded from
the hash on purpose).

Why it's a first-class artifact, not an internal detail:

- It serializes to JSON, so CI can diff plans across commits.
- It can be reviewed once and replayed: `dots plan > p.json` on one host,
  `dots apply --plan p.json` on another. Schema-version mismatches fail
  loudly rather than silently re-planning.
- The same shape backs `dots status`'s "last applied" receipt: the hash
  in `applied.toml` is the hash of the Plan that ran.

Steps carry an action verb (+ add, ~ change, - remove), a stable kind
(`bootstrap-nix`, `apply-profile`, …), a human summary, and the exact
shell command dots will run. A no-op Plan (zero steps) under --dry-run
exits 5; a non-empty Plan that the user declines exits 3.

See the `plan` package in apps/cli/internal/plan for the wire format.
