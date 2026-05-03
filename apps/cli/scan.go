package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// trackedPaths mirrors modules/scripts/backup.sh — keep in sync.
// Each entry is relative to $HOME and represents a path that Home
// Manager will project as a symlink.
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
