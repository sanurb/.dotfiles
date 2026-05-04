// Package rollback resolves the right "switch to a prior Home Manager
// generation" invocation for the host. nh is preferred when present
// because the codebase already commits to it for activation; the
// home-manager direct invocation is the fallback so a host without nh
// still has a path back. The package returns argv only — the caller
// owns subprocess execution (via internal/nix) and rendering.
package rollback

import (
	"errors"
	"os/exec"

	"github.com/sanurb/.dotfiles/apps/cli/internal/nix"
)

// ErrNoTool reports that neither nh nor home-manager is on PATH —
// Home Manager activation has not bootstrapped this host.
var ErrNoTool = errors.New("no rollback tool available: neither nh nor home-manager on PATH")

// Resolve returns the argv for rolling back to generation. An empty
// generation means "previous." On a host with neither tool installed,
// returns ErrNoTool.
func Resolve(generation string) ([]string, error) {
	return resolveWith(generation, exec.LookPath)
}

// resolveWith is the unit-test seam. lookPath substitutes for
// exec.LookPath so tests can simulate "only nh present," "only
// home-manager present," and "neither" without touching $PATH.
func resolveWith(generation string, lookPath func(string) (string, error)) ([]string, error) {
	if _, err := lookPath(nix.ToolNh); err == nil {
		return nhArgs(generation), nil
	}
	if _, err := lookPath(nix.ToolHomeManager); err == nil {
		return homeManagerArgs(generation), nil
	}
	return nil, ErrNoTool
}

func nhArgs(generation string) []string {
	args := []string{nix.ToolNh, "home", "rollback"}
	if generation != "" {
		args = append(args, generation)
	}
	return args
}

func homeManagerArgs(generation string) []string {
	if generation != "" {
		return []string{nix.ToolHomeManager, "--switch-generation", generation}
	}
	return []string{nix.ToolHomeManager, "--rollback"}
}
