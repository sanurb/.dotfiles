package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// syncMoonHooksSilent reconciles .git/hooks/{pre-commit,pre-push}
// with the declarative `vcs.hooks` block in .moon/workspace.yml.
// It is the enforcement edge of "Declare, Don't Script": a fresh
// clone, a hand-edited hook, or a stale hook from before the Moon
// migration all converge to the workspace spec on the next sync.
//
// Output is discarded — `moon sync hooks` is idempotent and chatty;
// the wizard owns the user-visible surface. Failure is swallowed for
// the same reason the prior treefmt pass was: a missing `moon` (e.g.
// pre-bootstrap) or a transient permission error on .git/hooks must
// not block a sync. The doctor's formatting check still gates deploy,
// so real drift surfaces there.
func syncMoonHooksSilent() {
	if _, err := exec.LookPath("moon"); err != nil {
		return
	}
	cmd := exec.Command("moon", "sync", "hooks")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

// run launches a child process inheriting stdio and propagates its
// exit code. Used for re-execing the dots binary itself (the wizard's
// hand-off to `dots deploy`) and for any future helper invocation
// where dropping stdin/out/err is undesirable. The cmd_apply path
// has its own teaching-error wrapper around `moon run`; this helper
// stays minimal so the caller decides on UX.
func run(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, name, "failed:", err)
		return 1
	}
	return 0
}
