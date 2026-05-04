# Stable exit-code contract

`dots` exposes a fixed exit-code surface that scripts branch on:
`0` Success, `1` Failure, `2` Misuse, `3` Declined, `4` PreFlight,
`5` NoOp, `130` Aborted. The codes are part of the public API. `3`
(user said no to a consent prompt) is deliberately distinct from `1`
(something failed) so wrapper retry-loops do not retry declined
consent, and `5` (already converged, nothing to do) is distinct from
`0` so CI can detect "no change" without parsing output.

## Consequences

Renumbering is a breaking change. New conditions get a new code;
existing codes do not get reassigned. The constants live in
`apps/cli/internal/exitcode/code.go`.
