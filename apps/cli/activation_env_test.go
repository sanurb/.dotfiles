package main

import (
	"maps"
	"strings"
	"testing"
)

func TestBuildActivationEnv(t *testing.T) {
	t.Run("when workspace is reachable, then NH_SHOW_ACTIVATION_LOGS is set and devenv profile bin leads PATH", func(t *testing.T) {
		shellEnv := buildShellEnv(map[string]string{"PATH": "/usr/bin:/bin"})
		workspace := "/Users/dev/dotfiles"

		got := asEnvMap(buildActivationEnv(shellEnv, workspace))

		want := map[string]string{
			"PATH":                    "/Users/dev/dotfiles/.devenv/profile/bin:/usr/bin:/bin",
			"HOME":                    "/Users/dev",
			"TERM":                    "xterm-256color",
			"NH_SHOW_ACTIVATION_LOGS": "1",
		}
		assertEnvEqual(t, got, want)
	})

	t.Run("when workspace is unresolvable, then NH_SHOW_ACTIVATION_LOGS is set but PATH is unchanged", func(t *testing.T) {
		shellEnv := []string{"PATH=/usr/bin", "FOO=bar"}

		got := asEnvMap(buildActivationEnv(shellEnv, ""))

		want := map[string]string{
			"PATH":                    "/usr/bin",
			"FOO":                     "bar",
			"NH_SHOW_ACTIVATION_LOGS": "1",
		}
		assertEnvEqual(t, got, want)
	})
}

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

func asEnvMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i > 0 {
			out[e[:i]] = e[i+1:]
		}
	}
	return out
}

func assertEnvEqual(t *testing.T, got, want map[string]string) {
	t.Helper()
	if !maps.Equal(got, want) {
		t.Errorf("env mismatch\n got: %v\nwant: %v", got, want)
	}
}
