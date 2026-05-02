// Package workspace resolves the dotfiles workspace root: the directory
// containing the flake and pinned-runtime declaration. It is the shared
// gate for every subcommand whose realization depends on the workspace
// being present (deploy, doctor, sync, scan, backup), and the helper
// behind any "is the user inside the workspace?" check elsewhere.
//
// A subcommand that runs without a workspace (install, version, help)
// must not call this package.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root walks up from the current working directory looking for the
// `.prototools` marker. MOON_WORKSPACE_ROOT short-circuits the walk
// when Moon already knows the answer (its tasks set this in their
// subprocess env). Returns an error when no workspace is reachable —
// callers decide whether to gate, fall back, or surface to the user.
func Root() (string, error) {
	if v := os.Getenv("MOON_WORKSPACE_ROOT"); v != "" {
		return v, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".prototools")); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("could not locate workspace root (no .prototools found)")
}
