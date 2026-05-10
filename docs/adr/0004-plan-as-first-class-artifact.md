# Plan as a first-class artifact

`dots plan` emits a content-addressed plan describing every step that
`dots apply` would run; `dots apply` either recomputes one or executes
a saved one only when its hash matches a fresh recomputation against
the current host. The plan/apply split exists because every state-
mutating step (install Nix, clone workspace, snapshot conflicts,
activate) is shown verbatim before consent — a user cannot approve
"the apply" without seeing exactly what it does.

## Considered Options

- **Just-do-it** (no plan; `apply` mutates immediately on `[y/N]`) —
  rejected because the user has to read source to know what their
  consent covers.
- **Terraform-style stateful plan** (a stored authoritative state
  file) — rejected; the plan is recomputed each run from observed
  state. Receipts (`applied.toml`) record what _did_ run, not what
  _should_.
