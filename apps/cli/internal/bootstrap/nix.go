package bootstrap

import (
	"fmt"
	"io"
	"os/exec"
)

// determinateInstaller is the exact command surfaced to the user
// before consent. The string MUST stay shell-safe: it is shown verbatim
// in the prompt and then handed to `sh -c`. Any change here changes
// what the user sees and approves.
const determinateInstaller = `curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install`

// NixInstallCommand returns the shell-safe Determinate Systems install
// command. Exposed so the plan renderer can echo the exact invocation
// that `bootstrap-nix` will run, keeping the plan and the consent
// prompt in lockstep.
func NixInstallCommand() string { return determinateInstaller }

// InstallNix runs the Determinate Systems installer without asking for
// consent. Callers (e.g. `dots apply --yes`) must have already obtained
// approval via their own UX before invoking this. The just-installed
// nix is NOT reachable in the calling process — same caveat as
// OfferNixInstall.
func InstallNix(out io.Writer, in io.Reader) error {
	cmd := exec.Command("sh", "-c", determinateInstaller)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nix installer: %w", err)
	}
	return nil
}

// NixPresent is true when `nix` is reachable on $PATH. The check is
// presence-only; version-floor enforcement lives in `dots doctor`.
func NixPresent() bool {
	_, err := exec.LookPath("nix")
	return err == nil
}

// OfferNixInstall asks the user to install Nix via the Determinate
// Systems installer. Returns (consented, err) — err is non-nil only if
// the consent prompt itself failed or the installer crashed; a "No"
// from the user is a clean (false, nil).
//
// The just-installed nix is NOT reachable in the calling process —
// the installer mutates files like ~/.bashrc and the kernel's PATH for
// future shells, but nothing in the current environment. Callers must
// instruct the user to open a new shell and re-run; do not attempt to
// continue execution here.
func OfferNixInstall(out io.Writer, in io.Reader) (bool, error) {
	fmt.Fprintln(out, "Nix is not installed.")
	fmt.Fprintln(out)
	consented, err := Confirm(out, in,
		"Install Nix using the Determinate Systems installer?",
		determinateInstaller)
	if err != nil || !consented {
		return false, err
	}
	cmd := exec.Command("sh", "-c", determinateInstaller)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return true, fmt.Errorf("nix installer: %w", err)
	}
	return true, nil
}
