// Package loginshell wires the pillar shell selected by the dots
// wizard onto the user's OS-level login shell. Without this, a user
// who picks fish in `dots install` ends up with a fish binary in
// ~/.nix-profile/bin but a `/bin/zsh` login shell — the symptom they
// describe as "fish was selected but did not end up installed."
//
// The package is split into two layers so the decision is pure and
// unit-testable: Decide computes what to do from inputs; Apply
// performs the side effects via injected runners.
package loginshell

import (
	"errors"
	"path/filepath"
	"strings"
)

// Kind enumerates the post-activation outcomes the decider can
// return. Each kind has a one-line Detail string for surfacing to the
// user without further branching at the call site.
type Kind int

const (
	// NoChange — the OS already reports target as the user's login
	// shell. The most common steady-state outcome.
	NoChange Kind = iota
	// Chsh — run `chsh -s TargetPath`. The only outcome that mutates.
	Chsh
	// SkipTargetMissing — target binary not on PATH. Typical on the
	// first activation that installs the shell: the binary is in the
	// new HM closure, but PATH for this process still points at the
	// pre-activation profile. The next `dots apply` finds it.
	SkipTargetMissing
	// SkipNotInEtcShells — target exists but /etc/shells doesn't list
	// it. chsh refuses to switch to a shell missing from /etc/shells;
	// editing it requires sudo, which we don't escalate from here.
	// User gets a one-line hint instead.
	SkipNotInEtcShells
	// SkipUnsupported — running on NixOS or another platform where
	// login shell is system-managed and chsh would either no-op or
	// be overridden on the next switch-config.
	SkipUnsupported
)

// Decision carries Kind plus the resolved TargetPath (when known) and
// a Detail string for user-facing output.
type Decision struct {
	Kind       Kind
	TargetPath string
	Detail     string
}

// Inputs is the bundle Decide consumes. Constructed once by Apply
// from real OS calls; constructed by tests with hand-rolled values.
type Inputs struct {
	// Target is the pillar shell name ("fish" | "zsh" | "nushell").
	Target string
	// CurrentShell is the absolute path the OS reports as the user's
	// login shell (output of `dscl . -read /Users/$USER UserShell` on
	// macOS, `getent passwd $USER` on Linux).
	CurrentShell string
	// Resolve maps a shell name to its absolute path on PATH, or ""
	// when not found. Mirrors exec.LookPath semantics.
	Resolve func(name string) string
	// EtcShells holds the lines of /etc/shells (already trimmed; "" /
	// "#"-prefixed lines filtered out by the caller).
	EtcShells []string
	// IsNixOS is true when running NixOS; on NixOS the login shell is
	// owned by users.users.<u>.shell at the system level and chsh is
	// either a no-op or gets reverted, so we skip.
	IsNixOS bool
}

// Decide computes the outcome from in. The function is total: every
// input combination yields a Decision, never a panic.
func Decide(in Inputs) Decision {
	if in.IsNixOS {
		return Decision{Kind: SkipUnsupported, Detail: "NixOS: set users.users.<you>.shell at the system level instead"}
	}

	binName := shellBinary(in.Target)
	if binName == "" {
		return Decision{Kind: SkipUnsupported, Detail: "unknown shell pillar value: " + in.Target}
	}

	target := ""
	if in.Resolve != nil {
		target = in.Resolve(binName)
	}
	if target == "" {
		return Decision{Kind: SkipTargetMissing, Detail: binName + " not on PATH yet — rerun after this activation lands"}
	}

	if samePath(in.CurrentShell, target) {
		return Decision{Kind: NoChange, TargetPath: target, Detail: "login shell already " + target}
	}

	if !etcShellsContains(in.EtcShells, target) {
		return Decision{
			Kind:       SkipNotInEtcShells,
			TargetPath: target,
			Detail:     target + " is not in /etc/shells; add it (sudo tee -a /etc/shells), then rerun `dots apply`",
		}
	}

	return Decision{Kind: Chsh, TargetPath: target, Detail: "chsh -s " + target}
}

// shellBinary maps a pillar value to the executable name. Intentionally
// duplicated from doctor.go's local helper so this package has zero
// dependencies on the rest of the binary — keeps it cheap to test.
func shellBinary(pillar string) string {
	switch pillar {
	case "fish":
		return "fish"
	case "zsh":
		return "zsh"
	case "nushell":
		return "nu"
	}
	return ""
}

// samePath compares two file paths as login-shell identifiers. We
// only collapse trivial differences (Clean) — symlink resolution is
// the caller's job because resolving requires syscalls. macOS often
// reports `/bin/zsh` while a fresh fish install lives at
// `/Users/<u>/.nix-profile/bin/fish`; those compare unequal as strings,
// which is the correct answer.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// etcShellsContains reports whether path appears in lines, comparing
// after Clean so a stray trailing slash doesn't cause a false miss.
func etcShellsContains(lines []string, path string) bool {
	want := filepath.Clean(path)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if filepath.Clean(line) == want {
			return true
		}
	}
	return false
}

// ErrChshFailed wraps a non-zero chsh exit so callers can distinguish
// "we tried, OS refused" from "we never tried."
var ErrChshFailed = errors.New("chsh failed")
