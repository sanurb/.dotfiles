package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// buildMoonProbe is the data factory for a moon resolution probe.
// Defaults reject every lookup ("nothing reachable") so each test
// declares only the deltas that matter for the layer it asserts.
func buildMoonProbe(overrides moonProbe) moonProbe {
	notFound := errors.New("not found")
	base := moonProbe{
		lookPath:  func(string) (string, error) { return "", notFound },
		statFile:  func(string) error { return notFound },
		workspace: func() (string, error) { return "", notFound },
	}
	if overrides.lookPath != nil {
		base.lookPath = overrides.lookPath
	}
	if overrides.statFile != nil {
		base.statFile = overrides.statFile
	}
	if overrides.workspace != nil {
		base.workspace = overrides.workspace
	}
	return base
}

func TestResolveMoon(t *testing.T) {
	t.Run("when moon is on PATH, then layer one wins and the cmd args lead with bare moon", func(t *testing.T) {
		probe := buildMoonProbe(moonProbe{
			lookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		})

		cmd, label := resolveMoon(probe)

		assertResolved(t, cmd, label, []string{"moon", "run", "modules:deploy"}, "moon run modules:deploy")
	})

	t.Run("when moon is missing from PATH but workspace has .proto/bin/moon, then layer two resolves to that absolute path", func(t *testing.T) {
		ws := "/Users/dev/dotfiles"
		want := filepath.Join(ws, ".proto", "bin", "moon")
		probe := buildMoonProbe(moonProbe{
			workspace: func() (string, error) { return ws, nil },
			statFile:  func(p string) error { return errIfMismatch(p, want) },
		})

		cmd, label := resolveMoon(probe)

		assertResolved(t, cmd, label,
			[]string{want, "run", "modules:deploy"},
			want+" run modules:deploy")
	})

	t.Run("when only nix is on PATH, then layer three falls back to nix develop", func(t *testing.T) {
		probe := buildMoonProbe(moonProbe{
			lookPath: func(name string) (string, error) {
				if name == "nix" {
					return "/usr/bin/nix", nil
				}
				return "", errors.New("not found")
			},
		})

		cmd, label := resolveMoon(probe)

		assertResolved(t, cmd, label,
			[]string{"nix", "develop", "-c", "moon", "run", "modules:deploy"},
			"nix develop -c moon run modules:deploy")
	})

	t.Run("when nothing is reachable, then resolver returns nil so the caller can render a teaching error", func(t *testing.T) {
		cmd, label := resolveMoon(buildMoonProbe(moonProbe{}))

		if cmd != nil || label != "" {
			t.Errorf("expected (nil, \"\"), got (%v, %q)", cmd, label)
		}
	})
}

func errIfMismatch(got, want string) error {
	if got == want {
		return nil
	}
	return errors.New("not found")
}

// assertResolved is the package's single comparison primitive for
// resolver outcomes. Bundles cmd args + label so a failure prints
// the whole tuple — no per-field drill-down.
func assertResolved(t *testing.T, cmd *exec.Cmd, label string, wantArgs []string, wantLabel string) {
	t.Helper()
	if cmd == nil {
		t.Fatalf("cmd was nil; expected args %v", wantArgs)
	}
	if !reflect.DeepEqual(cmd.Args, wantArgs) || label != wantLabel {
		t.Errorf("\n got:  args=%v label=%q\nwant: args=%v label=%q",
			cmd.Args, label, wantArgs, wantLabel)
	}
}
