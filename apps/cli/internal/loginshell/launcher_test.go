package loginshell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareResilientLoginShellTargetUsesStableProfilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	profileTarget := writeTestShell(t, filepath.Join(home, ".nix-profile", "bin"), "fish", "profile fish")

	launcher, err := prepareResilientLoginShellTarget("fish", "/nix/store/old-fish/bin/fish")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, loginShellLauncherDir, "fish"); launcher != want {
		t.Fatalf("launcher = %q, want %q", launcher, want)
	}
	body, err := os.ReadFile(launcher)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), profileTarget) {
		t.Fatalf("launcher does not target stable profile path %q:\n%s", profileTarget, body)
	}
}

func TestPrepareResilientLoginShellTargetLeavesSystemShellUnwrapped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const systemShell = "/bin/zsh"
	got, err := prepareResilientLoginShellTarget("zsh", systemShell)
	if err != nil {
		t.Fatal(err)
	}
	if got != systemShell {
		t.Fatalf("target = %q, want unwrapped system shell %q", got, systemShell)
	}
}

func TestWriteResilientLoginShellLauncherUsesTargetWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	target := writeTestShell(t, dir, "target", "target")
	fallback := writeTestShell(t, dir, "fallback", "fallback")
	launcher := filepath.Join(dir, "login-shells", "fish")

	if err := writeResilientLoginShellLauncher(launcher, target, fallback); err != nil {
		t.Fatal(err)
	}

	got, err := exec.Command(launcher).CombinedOutput()
	if err != nil {
		t.Fatalf("launcher failed: %v\n%s", err, got)
	}
	if strings.TrimSpace(string(got)) != "target" {
		t.Fatalf("launcher output = %q, want target", got)
	}
}

func TestWriteResilientLoginShellLauncherFallsBackWhenNixTargetIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	missingTarget := filepath.Join(dir, "unmounted-nix-store", "fish")
	fallback := writeTestShell(t, dir, "fallback", "fallback")
	launcher := filepath.Join(dir, "login-shells", "fish")

	if err := writeResilientLoginShellLauncher(launcher, missingTarget, fallback); err != nil {
		t.Fatal(err)
	}

	got, err := exec.Command(launcher).CombinedOutput()
	if err != nil {
		t.Fatalf("launcher failed instead of falling back: %v\n%s", err, got)
	}
	output := string(got)
	if !strings.Contains(output, "dots login shell: "+missingTarget+" unavailable") {
		t.Fatalf("launcher output %q missing unavailable-target diagnostic", output)
	}
	if !strings.HasSuffix(strings.TrimSpace(output), "fallback") {
		t.Fatalf("launcher output = %q, want fallback shell to run", output)
	}
}

func writeTestShell(t *testing.T, dir, name, output string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	body := "#!/bin/sh\nprintf '%s\\n' " + shellSingleQuote(output) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
