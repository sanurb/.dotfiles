package loginshell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

// Apply runs Decide against the live OS and, when the decision is
// Chsh, invokes `chsh -s <path>`. The function never aborts an
// activation: every non-Chsh outcome is reported via reporter and
// returns nil. A real chsh failure returns an error so the caller
// can surface it without losing context.
//
// reporter is the human-facing output sink. Pass os.Stderr in the
// `dots apply` flow; pass nil in tests that don't care about
// rendering.
func Apply(ctx context.Context, target string, reporter io.Writer) (Decision, error) {
	if reporter == nil {
		reporter = io.Discard
	}

	in := Inputs{
		Target:       target,
		CurrentShell: currentLoginShell(),
		Resolve: func(name string) string {
			path, err := exec.LookPath(name)
			if err != nil {
				return ""
			}
			return path
		},
		EtcShells: readEtcShells(),
		IsNixOS:   isNixOS(),
	}

	d := Decide(in)
	switch d.Kind {
	case Chsh:
		fmt.Fprintf(reporter, "login-shell: %s\n", d.Detail)
		if err := chsh(ctx, d.TargetPath, reporter); err != nil {
			return d, err
		}
	case RegisterShell:
		// The remaining work needs root (append to /etc/shells) plus a
		// chsh. If a human is driving and hasn't opted out, do both with
		// consent; otherwise print the copy/paste fix and move on so a
		// headless apply never blocks on a password prompt.
		if !canRegister() {
			fmt.Fprintf(reporter, "login-shell: %s\n", d.Detail)
			fmt.Fprintf(reporter, "login-shell: to finish, run: %s\n", RegisterHint(d.TargetPath))
			return d, nil
		}
		fmt.Fprintf(reporter, "login-shell: %s\n", d.Detail)
		if !confirm(reporter, d.TargetPath) {
			fmt.Fprintf(reporter, "login-shell: skipped; run later with: %s\n", RegisterHint(d.TargetPath))
			return d, nil
		}
		if err := registerInEtcShells(ctx, d.TargetPath, reporter); err != nil {
			// Don't fail the apply: report and leave the hint so the
			// user can finish by hand.
			fmt.Fprintf(reporter, "login-shell: could not register (%v); run manually: %s\n", err, RegisterHint(d.TargetPath))
			return d, nil
		}
		if err := chsh(ctx, d.TargetPath, reporter); err != nil {
			return d, err
		}
		fmt.Fprintf(reporter, "login-shell: %s is now your login shell (open a new terminal to use it)\n", d.TargetPath)
	case NoChange:
		fmt.Fprintf(reporter, "login-shell: %s\n", d.Detail)
	default:
		fmt.Fprintf(reporter, "login-shell: %s\n", d.Detail)
	}
	return d, nil
}

// chsh sets the OS login shell to path. When a target username is known
// we run `sudo chsh -s <path> <user>` so the change happens as root:
// bare `chsh -s` on macOS (chpass) pops a *second*, per-user PAM
// password prompt after the sudo we already did for /etc/shells, and
// that prompt aborts silently without a controlling TTY — leaving
// /etc/shells updated but the login shell unchanged. Running as root
// against the named user skips that prompt entirely. We fall back to
// bare chsh only when the username is unknown.
func chsh(ctx context.Context, path string, reporter io.Writer) error {
	var cmd *exec.Cmd
	if u := loginUser(); u != "" {
		cmd = exec.CommandContext(ctx, "sudo", "chsh", "-s", path, u)
	} else {
		cmd = exec.CommandContext(ctx, "chsh", "-s", path)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = reporter
	cmd.Stderr = reporter
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrChshFailed, err)
	}
	return nil
}

// loginUser resolves the account whose login shell is being changed.
// os/user is authoritative; $USER is a fallback for stripped
// environments. Empty means "unknown" — chsh then targets the caller.
func loginUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

// registerInEtcShells appends path to /etc/shells via sudo, idempotently
// (grep -qxF guards against a duplicate line and against a concurrent
// writer). sudo reads its password from the controlling TTY, so this
// works even though only the confirmation prompt uses os.Stdin.
func registerInEtcShells(ctx context.Context, path string, reporter io.Writer) error {
	fmt.Fprintf(reporter, "login-shell: registering %s in /etc/shells (sudo)\n", path)
	script := fmt.Sprintf("grep -qxF %q /etc/shells || printf '%%s\\n' %q >> /etc/shells", path, path)
	cmd := exec.CommandContext(ctx, "sudo", "sh", "-c", script)
	cmd.Stdin = os.Stdin
	cmd.Stdout = reporter
	cmd.Stderr = reporter
	return cmd.Run()
}

// RegisterHint is the copy/paste one-liner that finishes the login-shell
// switch by hand. Exported so `dots doctor` renders the exact command
// Apply would run, keeping a single source of truth. It uses
// `sudo chsh -s <path> <user>` rather than bare `chsh -s` so the change
// runs as root and avoids the second per-user PAM password prompt that
// silently aborts without a TTY (see chsh's doc comment).
func RegisterHint(path string) string {
	if u := loginUser(); u != "" {
		return fmt.Sprintf("echo %s | sudo tee -a /etc/shells && sudo chsh -s %s %s", path, path, u)
	}
	return fmt.Sprintf("echo %s | sudo tee -a /etc/shells && sudo chsh -s %s", path, path)
}

// canRegister reports whether Apply may attempt the privileged
// registration. It requires an interactive stdin (so sudo/chsh can
// prompt) and honors an explicit opt-out for users who manage
// /etc/shells themselves.
func canRegister() bool {
	if strings.EqualFold(os.Getenv("DOTS_REGISTER_SHELL"), "never") {
		return false
	}
	return stdinIsInteractive()
}

// stdinIsInteractive is a package var so tests can pin the TTY signal
// without depending on how the test runner wires stdin.
var stdinIsInteractive = interactiveStdin

// confirm asks the user before mutating /etc/shells and the login shell.
// Defaults to yes because the shell was already chosen in the wizard;
// DOTS_YES=1 (or DOTS_REGISTER_SHELL=always) skips the question for
// scripted-but-interactive installs. sudo still gates the actual change.
func confirm(reporter io.Writer, path string) bool {
	if os.Getenv("DOTS_YES") == "1" || strings.EqualFold(os.Getenv("DOTS_REGISTER_SHELL"), "always") {
		return true
	}
	fmt.Fprintf(reporter, "login-shell: make %s your login shell now? (adds it to /etc/shells via sudo) [Y/n] ", path)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true
	default:
		return false
	}
}

// interactiveStdin reports whether stdin is a terminal. A character
// device is the portable signal that a human can answer prompts; the
// headless stream path runs with stdin redirected, so this is false and
// Apply degrades to the printed hint instead of blocking.
func interactiveStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Probe computes the login-shell Decision from the live OS without any
// side effects. `dots doctor` uses it to surface the same outcome Apply
// would act on, so the divergence is visible before an apply runs.
func Probe(target string) Decision {
	return Decide(Inputs{
		Target:       target,
		CurrentShell: currentLoginShell(),
		Resolve: func(name string) string {
			path, err := exec.LookPath(name)
			if err != nil {
				return ""
			}
			return path
		},
		EtcShells: readEtcShells(),
		IsNixOS:   isNixOS(),
	})
}

// currentLoginShell asks the OS what the current user's login shell
// is. macOS exposes it via dscl; Linux via getent; both fall back to
// $SHELL when the platform-specific tool isn't reachable. Failures
// return "" so Decide treats them as "no current shell info" rather
// than guessing.
func currentLoginShell() string {
	switch runtime.GOOS {
	case "darwin":
		if u := os.Getenv("USER"); u != "" {
			out, err := exec.Command("dscl", ".", "-read", "/Users/"+u, "UserShell").Output()
			if err == nil {
				// Output: "UserShell: /bin/zsh"
				line := strings.TrimSpace(string(out))
				if i := strings.IndexByte(line, ':'); i >= 0 {
					return strings.TrimSpace(line[i+1:])
				}
			}
		}
	case "linux":
		if u := os.Getenv("USER"); u != "" {
			out, err := exec.Command("getent", "passwd", u).Output()
			if err == nil {
				// Output: "user:x:1000:1000:Name:/home/user:/bin/zsh"
				fields := strings.Split(strings.TrimSpace(string(out)), ":")
				if len(fields) >= 7 {
					return fields[6]
				}
			}
		}
	}
	return os.Getenv("SHELL")
}

// readEtcShells returns the lines of /etc/shells. Missing file is
// treated as an empty list so Decide reports RegisterShell — the
// actionable outcome — rather than a hard error.
func readEtcShells() []string {
	f, err := os.Open("/etc/shells")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// isNixOS reports whether this host is NixOS. The presence of
// /etc/NIXOS is the canonical detection used by NixOS itself
// (configuration.nix sets `environment.etc."NIXOS".text = "";`).
func isNixOS() bool {
	_, err := os.Stat("/etc/NIXOS")
	return err == nil
}
