package nix_test

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/nix"
)

// resolveTool resolves a POSIX coreutil against PATH. Hardcoded
// paths drift across distros (/bin/true vs /usr/bin/true), so the
// tests look it up at runtime. Skips the test when missing — a
// nix-develop shell and a stock Linux/macOS host both have these.
func resolveTool(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not on PATH: %v", name, err)
	}
	return path
}

func TestCmdRun_zeroExit_returnsNil(t *testing.T) {
	if err := (nix.Cmd{Name: resolveTool(t, "true")}).Run(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCmdRun_nonZeroExit_wrapsExitCode(t *testing.T) {
	err := nix.Cmd{Name: resolveTool(t, "false")}.Run(context.Background())

	var nerr *nix.Error
	if !errors.As(err, &nerr) {
		t.Fatalf("expected *nix.Error, got %T: %v", err, err)
	}
	if got := nerr.ExitCode(); got != 1 {
		t.Fatalf("expected exit code 1, got %d", got)
	}
}

func TestCmdRun_binaryMissing_returnsExitCodeMinusOne(t *testing.T) {
	err := nix.Cmd{Name: "/no/such/binary/anywhere"}.Run(context.Background())

	var nerr *nix.Error
	if !errors.As(err, &nerr) {
		t.Fatalf("expected *nix.Error, got %T: %v", err, err)
	}
	if got := nerr.ExitCode(); got != -1 {
		t.Fatalf("expected exit code -1 (never started), got %d", got)
	}
}

func TestCmdRun_ctxCancelled_returnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := nix.Cmd{Name: resolveTool(t, "true")}.Run(ctx)
	if err == nil {
		t.Fatal("expected non-nil error from cancelled ctx")
	}
}

func TestCmdString(t *testing.T) {
	tests := []struct {
		name string
		cmd  nix.Cmd
		want string
	}{
		{"name only", nix.Cmd{Name: "nh"}, "nh"},
		{"name + single arg", nix.Cmd{Name: "nh", Args: []string{"--version"}}, "nh --version"},
		{
			"full activation invocation",
			nix.Cmd{Name: "nh", Args: []string{"home", "switch", "-c", "aarch64-darwin", "."}},
			"nh home switch -c aarch64-darwin .",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cmd.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
