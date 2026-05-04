package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// trackedPaths is the canonical list of $HOME-relative paths that
// Home Manager projects as symlinks; collisions with real files/dirs
// at these locations need quarantining before activation. Each entry
// must match a `home.file."..."` source in modules/. `dots backup` is
// the only entry point and reads directly from this slice; a previous
// duplicate listing in modules/scripts/backup.sh was retired.
var trackedPaths = []string{
	".config/ghostty/config",
	".config/zellij/config.kdl",
	".config/fish/config.fish",
	".config/starship.toml",
	".config/nvim/init.lua",
	".config/git/config",
	".zshrc",
	".bashrc",
	".bash_profile",
}

type collision struct {
	rel  string
	abs  string
	kind string // "file" | "dir"
}

// findCollisions returns paths that exist as real files/dirs (not symlinks).
// A pre-existing symlink is assumed to be a previous Home Manager activation
// and is left alone — Home Manager will rewrite it.
func findCollisions(home string) ([]collision, error) {
	var out []collision
	for _, rel := range trackedPaths {
		abs := filepath.Join(home, rel)
		info, err := os.Lstat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("lstat %s: %w", abs, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		kind := "file"
		if info.IsDir() {
			kind = "dir"
		}
		out = append(out, collision{rel: rel, abs: abs, kind: kind})
	}
	return out, nil
}

func runScan() int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot resolve $HOME:", err)
		return 1
	}
	cs, err := findCollisions(home)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan failed:", err)
		return 1
	}
	if len(cs) == 0 {
		fmt.Println("✓ No brownfield collisions. Safe to deploy.")
		return 0
	}
	fmt.Printf("Found %d collision(s):\n", len(cs))
	for _, c := range cs {
		fmt.Printf("  • [%s] %s\n", c.kind, c.rel)
	}
	return 0
}
