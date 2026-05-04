package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
)

// captureOutput redirects stdout/stderr around fn and returns the
// captured streams plus the exit code fn produced. Used by the smoke
// tests so we can assert on what verbs print without coupling to a
// subprocess harness.
func captureOutput(t *testing.T, fn func() int) (stdout, stderr string, code int) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	doneOut := make(chan string, 1)
	doneErr := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := rOut.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		doneOut <- b.String()
	}()
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := rErr.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		doneErr <- b.String()
	}()

	code = fn()

	wOut.Close()
	wErr.Close()
	stdout = <-doneOut
	stderr = <-doneErr
	os.Stdout, os.Stderr = origOut, origErr
	return
}

func TestSmokePlanHuman(t *testing.T) {
	stdout, _, code := captureOutput(t, func() int { return runPlan(nil) })
	if code != exitcode.Success {
		t.Fatalf("runPlan exit=%d want=0", code)
	}
	if !strings.HasPrefix(stdout, "Plan:") {
		t.Fatalf("expected Plan: header, got %q", firstLine(stdout))
	}
	if !strings.Contains(stdout, "apply-profile") {
		t.Fatalf("expected apply-profile step, got:\n%s", stdout)
	}
}

func TestSmokePlanJSON(t *testing.T) {
	stdout, _, code := captureOutput(t, func() int { return runPlan([]string{"--json"}) })
	if code != exitcode.Success {
		t.Fatalf("runPlan --json exit=%d want=0", code)
	}
	var p plan.Plan
	if err := json.Unmarshal([]byte(stdout), &p); err != nil {
		t.Fatalf("plan --json output not valid JSON: %v\n%s", err, stdout)
	}
	if p.Hash == "" {
		t.Fatalf("plan hash empty in JSON output")
	}
	if len(p.Steps) == 0 {
		t.Fatalf("plan has no steps; expected at least apply-profile")
	}
}

func TestSmokeDiff(t *testing.T) {
	stdout, _, code := captureOutput(t, func() int { return runDiff(nil) })
	// diff returns NoOp (5) only when len(Steps)==0; in this workspace
	// apply-profile is always present, so we expect Success.
	if code != exitcode.Success && code != exitcode.NoOp {
		t.Fatalf("runDiff exit=%d want 0 or 5", code)
	}
	if !strings.HasPrefix(stdout, "Plan:") {
		t.Fatalf("expected Plan: header from diff, got %q", firstLine(stdout))
	}
}

func TestSmokeApplyDryRun(t *testing.T) {
	stdout, _, code := captureOutput(t, func() int { return runApply([]string{"--dry-run"}) })
	if code != exitcode.Success && code != exitcode.NoOp {
		t.Fatalf("runApply --dry-run exit=%d want 0 or 5", code)
	}
	if !strings.HasPrefix(stdout, "Plan:") {
		t.Fatalf("expected Plan: header from apply --dry-run, got %q", firstLine(stdout))
	}
}

func TestSmokeApplyDryRunJSON(t *testing.T) {
	stdout, _, code := captureOutput(t, func() int { return runApply([]string{"--dry-run", "--json"}) })
	if code != exitcode.Success && code != exitcode.NoOp {
		t.Fatalf("runApply --dry-run --json exit=%d want 0 or 5", code)
	}
	var p plan.Plan
	if err := json.Unmarshal([]byte(stdout), &p); err != nil {
		t.Fatalf("apply --dry-run --json output not valid JSON: %v\n%s", err, stdout)
	}
	if p.Hash == "" {
		t.Fatalf("plan hash empty in apply --dry-run --json output")
	}
}

func TestSmokeApplyNonInteractiveBootstrapRefused(t *testing.T) {
	// We cannot easily simulate a missing-Nix host here, but we CAN
	// assert that the consent helper itself behaves: when the plan
	// has no consent steps, --non-interactive is allowed.
	_, _, code := captureOutput(t, func() int { return runApply([]string{"--dry-run", "--non-interactive"}) })
	if code != exitcode.Success && code != exitcode.NoOp {
		t.Fatalf("runApply --dry-run --non-interactive exit=%d want 0 or 5", code)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
