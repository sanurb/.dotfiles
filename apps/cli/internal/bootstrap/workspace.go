package bootstrap

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// repoCloneURL is the canonical clone source for the workspace. Kept
// here rather than in main.go's URL block so the bootstrap package
// stays self-contained — main.go's gate-message constants and this
// constant must agree, but the duplication is one string and the
// callers are unrelated.
const repoCloneURL = "https://github.com/sanurb/.dotfiles"

// Target returns the workspace clone destination. DOTS_WORKSPACE wins
// if set; otherwise ~/.dotfiles.
func Target() (string, error) {
	if v := os.Getenv("DOTS_WORKSPACE"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, ".dotfiles"), nil
}

// CloneCommand returns the exact `git clone` invocation that
// CloneWorkspace would run for the current Target(). Exposed so the
// plan renderer can show the user what bootstrapping will do without
// duplicating the URL/target wiring.
func CloneCommand() (string, error) {
	target, err := Target()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("git clone %s %s", repoCloneURL, target), nil
}

// CloneWorkspace clones the dots repo into Target() without asking
// for consent. Callers (e.g. `dots apply --yes`) must have already
// obtained approval before invoking this. Returns the clone path on
// success.
func CloneWorkspace(out io.Writer, in io.Reader) (string, error) {
	target, err := Target()
	if err != nil {
		return "", err
	}
	git := exec.Command("git", "clone", repoCloneURL, target)
	git.Stdin = in
	git.Stdout = out
	git.Stderr = out
	if err := git.Run(); err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}
	return target, nil
}

// OfferWorkspaceClone asks the user to clone the dots repo into the
// target directory. Returns (path, err) where path is the clone
// destination on success and "" when the user declines or an error
// occurs. The caller is responsible for chdir'ing into path before
// continuing — the just-cloned tree is not on the current process's
// working directory.
func OfferWorkspaceClone(out io.Writer, in io.Reader) (string, error) {
	target, err := Target()
	if err != nil {
		return "", err
	}
	command := fmt.Sprintf("git clone %s %s", repoCloneURL, target)

	fmt.Fprintln(out, "No dotfiles workspace found.")
	fmt.Fprintln(out)
	consented, err := Confirm(out, in, "Clone the dots workspace?", command)
	if err != nil || !consented {
		return "", err
	}

	git := exec.Command("git", "clone", repoCloneURL, target)
	git.Stdin = in
	git.Stdout = out
	git.Stderr = out
	if err := git.Run(); err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}
	return target, nil
}
