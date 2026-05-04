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
	"sync"
)

// Root resolves the workspace root and caches the result for the
// process lifetime. The dispatcher gate, install's bootstrap branch,
// the next-step message, and several doctor checks all want the same
// answer; without caching, a single `dots install` would walk the
// directory tree four or five times. Use Reset() to invalidate the
// cache when the process mutates cwd mid-flow.
//
// MOON_WORKSPACE_ROOT short-circuits the walk when Moon already knows
// the answer (its tasks set this in their subprocess env). An error
// from Getwd is also cached — if cwd is unreadable once it is going
// to be unreadable for the rest of this process.
func Root() (string, error) {
	once.Do(func() { cachedRoot, cachedErr = computeRoot() })
	return cachedRoot, cachedErr
}

var (
	once       sync.Once
	cachedRoot string
	cachedErr  error
)

// Reset clears the cached lookup so the next Root() call re-evaluates
// against the current working directory. Use this *only* after the
// process mutates cwd mid-flow — the canonical case is `dots install`
// just cloned the workspace and chdir'd in. Without a reset, every
// subsequent Root() returns the pre-clone "no workspace" cache.
//
// Concurrency: callers must guarantee no other goroutine is inside
// Root() during the reset. In practice the dispatcher serializes both.
func Reset() {
	once = sync.Once{}
	cachedRoot = ""
	cachedErr = nil
}

func computeRoot() (string, error) {
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
