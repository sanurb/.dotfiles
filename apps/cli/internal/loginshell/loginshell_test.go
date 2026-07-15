package loginshell

import "testing"

// resolveMap returns a Resolve func backed by a string map; empty
// values mean "not on PATH" so a single map can describe both the
// presence and absence of a binary.
func resolveMap(m map[string]string) func(string) string {
	return func(name string) string { return m[name] }
}

func TestDecide(t *testing.T) {
	t.Run("when fish is selected, login shell is /bin/zsh, fish is on PATH, and /etc/shells lists it, then chsh is required", func(t *testing.T) {
		// Reproduces the exact scenario the user reports: fish picked
		// in the wizard, fish-the-binary installed by HM, but /bin/zsh
		// still the login shell. Without the post-activation chsh
		// step, dots leaves the OS in this divergent state.
		got := Decide(Inputs{
			Target:       "fish",
			CurrentShell: "/bin/zsh",
			Resolve:      resolveMap(map[string]string{"fish": "/Users/sanurb/.nix-profile/bin/fish"}),
			EtcShells:    []string{"# Comment", "/bin/zsh", "/Users/sanurb/.nix-profile/bin/fish"},
			IsNixOS:      false,
		})
		if got.Kind != Chsh {
			t.Fatalf("Kind = %v, want Chsh", got.Kind)
		}
		if got.TargetPath != "/Users/sanurb/.nix-profile/bin/fish" {
			t.Fatalf("TargetPath = %q, want fish absolute path", got.TargetPath)
		}
	})

	t.Run("when current login shell already matches target, then no chsh is run", func(t *testing.T) {
		got := Decide(Inputs{
			Target:       "fish",
			CurrentShell: "/usr/local/bin/fish",
			Resolve:      resolveMap(map[string]string{"fish": "/usr/local/bin/fish"}),
			EtcShells:    []string{"/usr/local/bin/fish"},
		})
		if got.Kind != NoChange {
			t.Fatalf("Kind = %v, want NoChange", got.Kind)
		}
	})

	t.Run("when target binary is not on PATH, then chsh is skipped (next apply will find it)", func(t *testing.T) {
		got := Decide(Inputs{
			Target:       "fish",
			CurrentShell: "/bin/zsh",
			Resolve:      resolveMap(map[string]string{}), // fish absent
			EtcShells:    []string{"/bin/zsh"},
		})
		if got.Kind != SkipTargetMissing {
			t.Fatalf("Kind = %v, want SkipTargetMissing", got.Kind)
		}
	})

	t.Run("when target is on PATH but not in /etc/shells, then registration is required", func(t *testing.T) {
		got := Decide(Inputs{
			Target:       "fish",
			CurrentShell: "/bin/zsh",
			Resolve:      resolveMap(map[string]string{"fish": "/Users/sanurb/.nix-profile/bin/fish"}),
			EtcShells:    []string{"/bin/zsh"},
		})
		if got.Kind != RegisterShell {
			t.Fatalf("Kind = %v, want RegisterShell", got.Kind)
		}
		if got.TargetPath == "" {
			t.Fatal("TargetPath should be populated so registration and the hint can name the path")
		}
	})

	t.Run("when running on NixOS, then chsh is skipped because login shell is system-managed", func(t *testing.T) {
		got := Decide(Inputs{
			Target:       "fish",
			CurrentShell: "/run/current-system/sw/bin/bash",
			Resolve:      resolveMap(map[string]string{"fish": "/run/current-system/sw/bin/fish"}),
			EtcShells:    []string{"/run/current-system/sw/bin/fish"},
			IsNixOS:      true,
		})
		if got.Kind != SkipUnsupported {
			t.Fatalf("Kind = %v, want SkipUnsupported", got.Kind)
		}
	})

	t.Run("when pillar value is unknown, then chsh is skipped", func(t *testing.T) {
		got := Decide(Inputs{Target: "elvish", CurrentShell: "/bin/zsh"})
		if got.Kind != SkipUnsupported {
			t.Fatalf("Kind = %v, want SkipUnsupported for unknown pillar", got.Kind)
		}
	})

	t.Run("when /etc/shells contains comments and blank lines, then they are ignored", func(t *testing.T) {
		got := Decide(Inputs{
			Target:       "fish",
			CurrentShell: "/bin/zsh",
			Resolve:      resolveMap(map[string]string{"fish": "/usr/local/bin/fish"}),
			EtcShells:    []string{"# top comment", "", "  ", "/bin/sh", "/usr/local/bin/fish"},
		})
		if got.Kind != Chsh {
			t.Fatalf("Kind = %v, want Chsh after filtering noise", got.Kind)
		}
	})

	t.Run("when /etc/shells lists target with a trailing slash, then it still matches", func(t *testing.T) {
		// Defensive against stray sysadmin edits — Clean collapses the
		// suffix so the equality check is total over real-world values.
		got := Decide(Inputs{
			Target:       "fish",
			CurrentShell: "/bin/zsh",
			Resolve:      resolveMap(map[string]string{"fish": "/usr/local/bin/fish"}),
			EtcShells:    []string{"/usr/local/bin/fish/"},
		})
		if got.Kind != Chsh {
			t.Fatalf("Kind = %v, want Chsh", got.Kind)
		}
	})

	t.Run("when nushell is selected, then the binary name resolves to nu", func(t *testing.T) {
		got := Decide(Inputs{
			Target:       "nushell",
			CurrentShell: "/bin/zsh",
			Resolve:      resolveMap(map[string]string{"nu": "/usr/local/bin/nu"}),
			EtcShells:    []string{"/usr/local/bin/nu"},
		})
		if got.Kind != Chsh || got.TargetPath != "/usr/local/bin/nu" {
			t.Fatalf("got = %+v, want Chsh /usr/local/bin/nu", got)
		}
	})
}
