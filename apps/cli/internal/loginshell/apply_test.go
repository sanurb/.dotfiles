package loginshell

import (
	"strings"
	"testing"
)

func TestRegisterHint(t *testing.T) {
	// The hint is a copy/paste one-liner; it must name the exact path
	// twice (etc/shells + chsh) so a user can finish by hand.
	got := RegisterHint("/Users/sanurb/.nix-profile/bin/fish")
	for _, want := range []string{"/etc/shells", "chsh -s", "/Users/sanurb/.nix-profile/bin/fish"} {
		if !strings.Contains(got, want) {
			t.Fatalf("hint %q missing %q", got, want)
		}
	}
}

// withTTY pins the interactivity probe for a test and restores it after.
func withTTY(t *testing.T, interactive bool) {
	t.Helper()
	prev := stdinIsInteractive
	stdinIsInteractive = func() bool { return interactive }
	t.Cleanup(func() { stdinIsInteractive = prev })
}

func TestCanRegister_OptOut(t *testing.T) {
	// DOTS_REGISTER_SHELL=never is an unconditional opt-out for users
	// who own /etc/shells themselves — never escalate, even on a TTY.
	withTTY(t, true)
	t.Setenv("DOTS_REGISTER_SHELL", "never")
	if canRegister() {
		t.Fatal("canRegister() = true with DOTS_REGISTER_SHELL=never, want false")
	}
}

func TestCanRegister_Headless(t *testing.T) {
	// Without a TTY, canRegister must be false — the guarantee that a
	// headless apply never blocks on sudo/chsh prompts and degrades to
	// the printed hint instead.
	withTTY(t, false)
	t.Setenv("DOTS_REGISTER_SHELL", "")
	if canRegister() {
		t.Fatal("canRegister() = true without a TTY, want false")
	}
}

func TestCanRegister_Interactive(t *testing.T) {
	// A TTY with no opt-out is the green light to offer registration.
	withTTY(t, true)
	t.Setenv("DOTS_REGISTER_SHELL", "")
	if !canRegister() {
		t.Fatal("canRegister() = false on a TTY without opt-out, want true")
	}
}

func TestConfirm_NonInteractiveOverrides(t *testing.T) {
	// DOTS_YES and DOTS_REGISTER_SHELL=always both short-circuit the
	// question without touching stdin, so scripted-but-interactive
	// installs proceed (sudo still gates the real change).
	t.Setenv("DOTS_YES", "1")
	if !confirm(nil, "/x/fish") {
		t.Fatal("confirm() = false with DOTS_YES=1, want true")
	}
	t.Setenv("DOTS_YES", "")
	t.Setenv("DOTS_REGISTER_SHELL", "always")
	if !confirm(nil, "/x/fish") {
		t.Fatal("confirm() = false with DOTS_REGISTER_SHELL=always, want true")
	}
}
