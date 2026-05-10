# dots explain: exit-codes

The numeric exit codes below are part of the dots CLI's stable
contract. Scripts may branch on them.

0 Success.
The verb completed and any side-effects landed cleanly.

1 Failure.
A generic runtime failure: a subprocess returned non-zero, a write
failed, a parse failed. The accompanying stderr message tells you
what; the cause is not one of the more specific codes below.

2 Misuse.
Usage error: unknown verb, bad flag, a prompt was needed but
--non-interactive (or CI=1) is set, or a stub verb was invoked.

3 Declined.
The user explicitly answered "no" to a confirmation prompt. This
is distinct from failure (1). A wrapper script that retries on
failure should NOT retry on 3.

4 PreFlight.
A precondition wasn't met: doctor reported drift, a workspace was
required and absent, a saved Plan's schema mismatches this binary.
The fix is a setup step, not a retry.

5 NoOp.
`apply --dry-run` (or equivalent) computed an empty Plan: the host
is already converged. Distinct from 0 so CI can detect "nothing
changed" without parsing output.

130 Aborted.
SIGINT (Ctrl-C) or wizard abort. Unix convention; dots inherits it.

These constants live in apps/cli/internal/exitcode/code.go.
