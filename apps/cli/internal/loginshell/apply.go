package loginshell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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
		cmd := exec.CommandContext(ctx, "chsh", "-s", d.TargetPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = reporter
		cmd.Stderr = reporter
		if err := cmd.Run(); err != nil {
			return d, fmt.Errorf("%w: %v", ErrChshFailed, err)
		}
	case NoChange:
		fmt.Fprintf(reporter, "login-shell: %s\n", d.Detail)
	default:
		fmt.Fprintf(reporter, "login-shell: %s\n", d.Detail)
	}
	return d, nil
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
// treated as an empty list so Decide reports SkipNotInEtcShells —
// the actionable outcome — rather than a hard error.
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
