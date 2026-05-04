package plan

import "testing"

func TestHostNixIdent(t *testing.T) {
	t.Run("when host is darwin arm64, then ident is aarch64-darwin", func(t *testing.T) {
		h := Host{OS: "darwin", Arch: "arm64"}

		got := h.NixIdent()

		assertString(t, got, "aarch64-darwin")
	})

	t.Run("when host is linux amd64, then ident is x86_64-linux", func(t *testing.T) {
		h := Host{OS: "linux", Arch: "amd64"}

		got := h.NixIdent()

		assertString(t, got, "x86_64-linux")
	})

	t.Run("when host arch is unmapped, then ident is empty so the bash fallback can run nix eval", func(t *testing.T) {
		h := Host{OS: "plan9", Arch: "riscv64"}

		got := h.NixIdent()

		assertString(t, got, "")
	})
}

func assertString(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
