package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// runBackup quarantines real files that would collide with Home Manager
// symlinks. When skipPrompt is false, it shells out to `gum confirm` so
// the user explicitly opts in to the destructive move.
func runBackup(skipPrompt bool) int {
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
		fmt.Println("✓ Nothing to back up.")
		return 0
	}

	dest := filepath.Join(home, ".dots_backups", time.Now().Format("20060102_150405"))

	fmt.Printf("Will move %d path(s) → %s\n", len(cs), dest)
	for _, c := range cs {
		fmt.Printf("  • %s\n", c.rel)
	}

	if !skipPrompt && !gumConfirm("Proceed?") {
		fmt.Println("Aborted by user.")
		return 1
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir backup dir:", err)
		return 1
	}
	for _, c := range cs {
		dst := filepath.Join(dest, c.rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "mkdir parent:", err)
			return 1
		}
		if err := os.Rename(c.abs, dst); err != nil {
			fmt.Fprintln(os.Stderr, "move", c.rel, ":", err)
			return 1
		}
	}
	fmt.Printf("✓ Backed up %d path(s) → %s\n", len(cs), dest)
	return 0
}

// gumConfirm shells out to `gum confirm`. If gum is missing (e.g. CI),
// fall back to false — explicit consent only.
func gumConfirm(prompt string) bool {
	if _, err := exec.LookPath("gum"); err != nil {
		return false
	}
	cmd := exec.Command("gum", "confirm", prompt)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run() == nil
}
