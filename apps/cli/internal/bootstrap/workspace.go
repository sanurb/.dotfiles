package bootstrap

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// repoCloneURL is the canonical clone source for the workspace. Kept
// here rather than in main.go's URL block so the bootstrap package
// stays self-contained — main.go's gate-message constants and this
// constant must agree, but the duplication is one string and the
// callers are unrelated.
const repoCloneURL = "https://github.com/sanurb/.dotfiles"

// Target returns the workspace clone destination. DOTS_WORKSPACE wins
// if set; otherwise ~/.dotfiles. Delegates to workspace.DefaultRoot so
// the clone destination and the workspace.Root() cwd-miss fallback
// resolve the same path — if they drifted, dots could clone into one
// directory while looking for the workspace in another.
func Target() (string, error) {
	return workspace.DefaultRoot()
}

// alreadyCloned reports whether target is an existing dotfiles
// workspace (the .prototools marker is present). Used to make the
// clone idempotent: a target that is already the workspace is success,
// not a fatal `git clone: destination already exists`.
func alreadyCloned(target string) bool {
	_, err := os.Stat(filepath.Join(target, ".prototools"))
	return err == nil
}

// isNonEmptyDir reports whether path exists and contains at least one
// entry. `git clone` refuses a non-empty destination with exit 128;
// probing first lets us return an actionable error instead.
func isNonEmptyDir(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
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
	if done, err := skipCloneIfPresent(out, target); done || err != nil {
		return target, err
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

// skipCloneIfPresent inspects target before a clone and returns
// (done=true) when cloning must be skipped:
//   - target is already the workspace → success, no clone (idempotent).
//   - target exists non-empty but is NOT the workspace → error, because
//     `git clone` would abort with exit 128 and a cryptic message.
//
// (done=false, nil) means the destination is absent or empty and the
// caller should proceed with the clone.
func skipCloneIfPresent(out io.Writer, target string) (bool, error) {
	if alreadyCloned(target) {
		fmt.Fprintf(out, "Workspace already present at %s — skipping clone.\n", target)
		return true, nil
	}
	if isNonEmptyDir(target) {
		return true, fmt.Errorf(
			"%s already exists and is not a dots workspace (no .prototools); "+
				"move it aside or set DOTS_WORKSPACE to a fresh path, then re-run",
			target,
		)
	}
	return false, nil
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
	// A workspace can exist at the canonical target even when the
	// caller's cwd-based probe missed it (running `dots` from outside
	// ~/.dotfiles). Detect that here so we adopt the existing checkout
	// instead of offering a clone that git would reject with exit 128.
	if done, err := skipCloneIfPresent(out, target); done || err != nil {
		if err != nil {
			return "", err
		}
		return target, nil
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
