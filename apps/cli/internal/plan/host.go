package plan

import "os"

// osHostname is split out so tests can stub it without monkey-patching
// the runtime. The error from os.Hostname is intentionally swallowed
// at the caller — a missing hostname is not a reason to refuse to
// emit a plan; downstream tooling reads the field as best-effort.
func osHostname() (string, error) {
	return os.Hostname()
}

// nixArchByGoarch maps Go's GOARCH tokens to Nix's prefix tokens.
// Closed table — adding a new architecture is a one-line edit here
// and a new test row in TestHostNixIdent. ADR-0011 records the
// supported-platform scope (macOS + Linux only).
var nixArchByGoarch = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
}

// NixIdent returns the Nix system identifier for this host (e.g.,
// "aarch64-darwin", "x86_64-linux") — the key flake.homeConfigurations
// is indexed by. Returns "" for an unknown architecture so callers can
// distinguish "platform unsupported" from "Linux/Darwin with valid
// arch"; modules:deploy's bash task falls back to `nix eval` when
// this is empty so a user on an unmapped arch still has a path home.
func (h Host) NixIdent() string {
	arch, ok := nixArchByGoarch[h.Arch]
	if !ok {
		return ""
	}
	return arch + "-" + h.OS
}
