package main

import (
	"maps"
	"strings"
	"testing"
)

func TestBuildMoonEnv(t *testing.T) {
	t.Run("when shell env carries PATH and a home dir, then output mirrors devenv.nix activation and exports DOTS_NIX_SYSTEM", func(t *testing.T) {
		shellEnv := buildShellEnv(map[string]string{"PATH": "/usr/bin:/bin"})
		workspace := "/Users/dev/dotfiles"
		home := "/Users/dev"
		nixSys := "aarch64-darwin"

		got := asEnvMap(buildMoonEnv(shellEnv, workspace, home, nixSys))

		want := map[string]string{
			"PATH":            "/Users/dev/dotfiles/.proto/shims:/Users/dev/dotfiles/.proto/bin:/Users/dev/dotfiles/.devenv/profile/bin:/Users/dev/.cargo/bin:/usr/bin:/bin",
			"HOME":            "/Users/dev",
			"TERM":            "xterm-256color",
			"PROTO_HOME":      "/Users/dev/dotfiles/.proto",
			"DOTS_NIX_SYSTEM": "aarch64-darwin",
		}
		assertEnvEqual(t, got, want)
	})

	t.Run("when shell env has no PATH, then PATH is synthesized from the activation dirs alone", func(t *testing.T) {
		shellEnv := []string{"FOO=bar"}
		workspace := "/Users/dev/dotfiles"
		home := "/Users/dev"
		nixSys := "x86_64-linux"

		got := asEnvMap(buildMoonEnv(shellEnv, workspace, home, nixSys))

		want := map[string]string{
			"PATH":            "/Users/dev/dotfiles/.proto/shims:/Users/dev/dotfiles/.proto/bin:/Users/dev/dotfiles/.devenv/profile/bin:/Users/dev/.cargo/bin",
			"FOO":             "bar",
			"PROTO_HOME":      "/Users/dev/dotfiles/.proto",
			"DOTS_NIX_SYSTEM": "x86_64-linux",
		}
		assertEnvEqual(t, got, want)
	})

	t.Run("when home dir is unresolvable, then cargo bin is omitted instead of resolving to /.cargo/bin", func(t *testing.T) {
		shellEnv := []string{"PATH=/usr/bin"}
		workspace := "/Users/dev/dotfiles"
		nixSys := "aarch64-linux"

		got := asEnvMap(buildMoonEnv(shellEnv, workspace, "", nixSys))

		want := map[string]string{
			"PATH":            "/Users/dev/dotfiles/.proto/shims:/Users/dev/dotfiles/.proto/bin:/Users/dev/dotfiles/.devenv/profile/bin:/usr/bin",
			"PROTO_HOME":      "/Users/dev/dotfiles/.proto",
			"DOTS_NIX_SYSTEM": "aarch64-linux",
		}
		assertEnvEqual(t, got, want)
	})
}

func TestNixSystem(t *testing.T) {
	t.Run("when GOARCH is arm64 on darwin, then identifier is aarch64-darwin", func(t *testing.T) {
		got := nixSystem("darwin", "arm64")

		assertStringEqual(t, got, "aarch64-darwin")
	})

	t.Run("when GOARCH is amd64 on linux, then identifier is x86_64-linux", func(t *testing.T) {
		got := nixSystem("linux", "amd64")

		assertStringEqual(t, got, "x86_64-linux")
	})

	t.Run("when GOARCH is unknown, then identifier is empty so the bash fallback can run nix eval", func(t *testing.T) {
		got := nixSystem("plan9", "riscv64")

		assertStringEqual(t, got, "")
	})
}

func assertStringEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// buildShellEnv is the data factory for a parent shell's environment.
// Defaults look like a bare login shell; overrides replace specific
// keys so each test reads as the minimal delta from a normal shell.
func buildShellEnv(overrides map[string]string) []string {
	base := map[string]string{
		"PATH": "/usr/bin:/bin",
		"HOME": "/Users/dev",
		"TERM": "xterm-256color",
	}
	for k, v := range overrides {
		base[k] = v
	}
	out := make([]string, 0, len(base))
	for k, v := range base {
		out = append(out, k+"="+v)
	}
	return out
}

// asEnvMap projects an env slice ("KEY=VAL") into a map for whole-value
// comparison. Order-insensitive — env keys aren't ordered semantically.
func asEnvMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i > 0 {
			out[e[:i]] = e[i+1:]
		}
	}
	return out
}

// assertEnvEqual is the assertion helper. The if-Errorf shape is the
// stdlib testing idiom that maps to expect().toEqual() — keeping it in
// one place lets the tests themselves stay flat.
func assertEnvEqual(t *testing.T, got, want map[string]string) {
	t.Helper()
	if !maps.Equal(got, want) {
		t.Errorf("env mismatch\n got: %v\nwant: %v", got, want)
	}
}
