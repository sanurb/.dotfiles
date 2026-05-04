// Package e2e holds system-wide end-to-end tests for the dots CLI.
// Tests build the binary, exec it as a subprocess, and assert against
// real stdout/stderr/exit code/side-effects — exactly what a user
// running dots from a terminal would observe.
//
// E2E tests are intentionally few and slow. Each one covers a flow we
// have actively had a bug in (ADR-0010/0012: bootstrap propagation,
// v0.4.3: moon's subprocess environment). They are the safety net
// that catches regressions our unit tests cannot, because the bugs
// live at the process boundary, not inside any one Go function.
package e2e
