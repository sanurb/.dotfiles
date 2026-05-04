// Package nix is the single subprocess seam for dots → Nix-tool
// invocations on the activation path: `nh home switch`, `nh home
// rollback`, and `home-manager` fallbacks. One seam means env shape,
// exit-code propagation, and command-printing have one place to fix.
// Metadata probes (e.g. `nh --version`) stay out of scope; this seam
// is for streamed-stdio activation calls.
//
// Callers describe the invocation as a Cmd value and call Run; the
// package owns os/exec construction so other Go files in the CLI do
// not need to import os/exec for Nix-tool subprocesses.
package nix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Tool names used by the activation seam. Kept here because every
// caller that resolves, lints, or printable-renders one of these
// tools imports this package already.
const (
	ToolNh          = "nh"
	ToolHomeManager = "home-manager"
)

// Cmd describes a Nix-tool subprocess on the activation path. Zero
// values are valid: a Cmd with only Name+Args set inherits the parent
// process env and runs with nil stdio (subprocess output discarded).
type Cmd struct {
	// Name is the executable. Absolute paths are used verbatim;
	// bare names resolve through the os/exec PATH lookup at Run
	// time. Callers that have already resolved a path against an
	// augmented env (see internal/activation.LookPathIn) should pass
	// the absolute result here so the lookup is not redone against a
	// different PATH.
	Name string
	Args []string

	// Env, when non-nil, replaces the parent process env wholesale.
	// nil means "inherit". Mirrors os/exec.Cmd.Env semantics.
	Env []string

	// Dir, when non-empty, sets the subprocess working directory.
	Dir string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// String formats c as a single shell-readable line. Best-effort: args
// containing spaces or shell metacharacters are not quoted, since our
// activation-path invocations have predictable, no-space tokens. Use
// for audit / dry-run output, not for clipboard re-execution.
func (c Cmd) String() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	return c.Name + " " + strings.Join(c.Args, " ")
}

// Run executes c with ctx-bounded lifetime and returns nil on a
// zero-exit completion. Any failure — non-zero exit, missing binary,
// fork error, ctx cancellation — is wrapped in *Error so callers get
// uniform unwrapping via errors.As without importing os/exec.
func (c Cmd) Run(ctx context.Context) error {
	p := exec.CommandContext(ctx, c.Name, c.Args...)
	p.Env = c.Env
	p.Dir = c.Dir
	p.Stdin = c.Stdin
	p.Stdout = c.Stdout
	p.Stderr = c.Stderr
	if err := p.Run(); err != nil {
		return &Error{Name: c.Name, Args: c.Args, Err: err}
	}
	return nil
}

// Error is a failed Nix-tool invocation. It carries enough context
// (binary, args) for call sites to render their own user-facing
// message without re-wrapping. Use errors.As to match.
type Error struct {
	Name string
	Args []string
	Err  error
}

func (e *Error) Error() string {
	if code, ok := exitCodeOf(e.Err); ok {
		return fmt.Sprintf("%s exited %d", e.Name, code)
	}
	return fmt.Sprintf("%s: %v", e.Name, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode returns the subprocess exit code when the process ran to
// completion. Returns -1 when the subprocess never started — a binary
// lookup failure, a fork error, or a ctx cancellation before exec —
// so callers can distinguish "exited with code N" (>=0) from
// "couldn't run at all" (<0) without further unwrapping.
func (e *Error) ExitCode() int {
	if code, ok := exitCodeOf(e.Err); ok {
		return code
	}
	return -1
}

func exitCodeOf(err error) (int, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), true
	}
	return 0, false
}

// IsExit reports the subprocess exit code when err represents a
// process that ran to completion. ok==true only when err is *Error
// and the underlying process produced a non-negative exit code.
// Callers branching on "did the binary actually run" should prefer
// this over hand-rolling errors.As + ExitCode() checks.
func IsExit(err error) (int, bool) {
	var ne *Error
	if errors.As(err, &ne) {
		if code := ne.ExitCode(); code >= 0 {
			return code, true
		}
	}
	return -1, false
}
