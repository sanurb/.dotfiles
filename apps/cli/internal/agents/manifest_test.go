package agents

import (
	"path/filepath"
	"strings"
	"testing"
)

const validBOM = `
schema_version = 1

[agents.herdr]
provider = "github-release"
version  = "0.8.2"
repo     = "herdrdev/herdr"
asset    = "herdr-{os}-{arch}"
probe    = "herdr --version"

[agents.herdr.sha256]
macos-aarch64 = "a5d4f4d504d8b309c91f811050559300faba31258425f53c50852fc96f6ae574"

[agents.pi]
provider = "bun"
version  = "0.85.1"
package  = "@earendil-works/pi-coding-agent"
probe    = "pi --version"

[agents.claude-code]
provider = "native"
probe    = "claude --version"
`

func parseString(t *testing.T, s string) (Manifest, error) {
	t.Helper()
	return parse(strings.NewReader(s))
}

func TestParseValid(t *testing.T) {
	m, err := parseString(t, validBOM)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	// Rows are sorted by name regardless of file order, so every
	// surface that iterates the manifest is deterministic.
	if got, want := m.Names(), []string{"claude-code", "herdr", "pi"}; !equal(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
	herdr, ok := m.Lookup("herdr")
	if !ok {
		t.Fatal("Lookup(herdr) not found")
	}
	if herdr.Provider != ProviderGitHubRelease || herdr.Version != "0.8.2" {
		t.Errorf("herdr = %+v", herdr)
	}
	if len(herdr.SHA256) != 1 {
		t.Errorf("herdr.SHA256 = %v, want one entry", herdr.SHA256)
	}
	if !herdr.Managed() {
		t.Error("github-release agent should be Managed()")
	}
	claude, _ := m.Lookup("claude-code")
	if claude.Managed() {
		t.Error("native agent must not be Managed()")
	}
}

// A hand-edited file must not silently ignore a typo. internal/state
// tolerates unknown keys because that file is machine-written; this one
// is not, and a dropped `verison` would leave an agent unpinned while
// the file reads as if it were pinned.
func TestParseRejectsUnknownKeys(t *testing.T) {
	cases := map[string]string{
		"unknown agent key":  "schema_version = 1\n[agents.x]\nverison = \"1.0.0\"\n",
		"unknown top-level":  "schema_version = 1\nnotakey = \"x\"\n",
		"unknown section":    "schema_version = 1\n[tools.x]\nprovider = \"bun\"\n",
		"unknown sub-table":  "schema_version = 1\n[agents.x]\n[agents.x.sha512]\na = \"b\"\n",
		"orphan sub-table":   "schema_version = 1\n[agents.x.sha256]\na = \"b\"\n",
		"duplicate agent":    "schema_version = 1\n[agents.x]\n[agents.x]\n",
		"unquoted value":     "schema_version = 1\n[agents.x]\nversion = 1.0.0\n",
		"missing equals":     "schema_version = 1\n[agents.x]\nprovider\n",
		"non-integer schema": "schema_version = \"1\"\n",
		"too deeply nested":  "schema_version = 1\n[agents.x.sha256.y]\na = \"b\"\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseString(t, src); err == nil {
				t.Fatalf("expected a parse error for %q", src)
			}
		})
	}
}

func TestValidateRejectsBadRows(t *testing.T) {
	const digest = "a5d4f4d504d8b309c91f811050559300faba31258425f53c50852fc96f6ae574"
	cases := map[string]struct {
		agent Agent
		want  string
	}{
		"native with version": {
			Agent{Name: "x", Provider: ProviderNative, Version: "1.0.0", Probe: "x --version"},
			"never be enforced",
		},
		"unknown provider": {
			Agent{Name: "x", Provider: "brew", Probe: "x --version"},
			"unknown provider",
		},
		"missing provider": {
			Agent{Name: "x", Probe: "x --version"},
			"provider is required",
		},
		"missing probe": {
			Agent{Name: "x", Provider: ProviderNative},
			"probe is required",
		},
		"bun without package": {
			Agent{Name: "x", Provider: ProviderBun, Version: "1.0.0", Probe: "x --version"},
			"package is required",
		},
		"bun without version": {
			Agent{Name: "x", Provider: ProviderBun, Package: "p", Probe: "x --version"},
			"version is required",
		},
		"release without digests": {
			Agent{Name: "x", Provider: ProviderGitHubRelease, Version: "1.0.0", Repo: "o/r", Asset: "a", Probe: "x --version"},
			"sha256",
		},
		"release with bad repo": {
			Agent{
				Name: "x", Provider: ProviderGitHubRelease, Version: "1.0.0", Repo: "justname", Asset: "a", Probe: "x --version",
				SHA256: map[string]string{"macos-aarch64": digest},
			},
			"owner/name",
		},
		"release with short digest": {
			Agent{
				Name: "x", Provider: ProviderGitHubRelease, Version: "1.0.0", Repo: "o/r", Asset: "a", Probe: "x --version",
				SHA256: map[string]string{"macos-aarch64": "abc"},
			},
			"64-character hex",
		},
		"release with non-hex digest": {
			Agent{
				Name: "x", Provider: ProviderGitHubRelease, Version: "1.0.0", Repo: "o/r", Asset: "a", Probe: "x --version",
				SHA256: map[string]string{"macos-aarch64": strings.Repeat("z", 64)},
			},
			"64-character hex",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.agent.validate()
			if err == nil {
				t.Fatalf("expected a validation error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestValidateRejectsSchemaMismatch(t *testing.T) {
	m := Manifest{SchemaVersion: SchemaVersion + 1, Agents: []Agent{{
		Name: "x", Provider: ProviderNative, Probe: "x --version",
	}}}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected a schema_version error, got %v", err)
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	m, found, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Error("found = true for an absent file")
	}
	if m.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", m.SchemaVersion, SchemaVersion)
	}
}

// The shipped BOM is the artifact every host converges against; a typo
// in it breaks `dots agents` on every machine at once. Parsing it here
// makes that a build failure rather than a runtime surprise.
func TestShippedManifestIsValid(t *testing.T) {
	m, found, err := Load(filepath.Join("..", "..", "..", "..", RelPath))
	if err != nil {
		t.Fatalf("load shipped %s: %v", RelPath, err)
	}
	if !found {
		t.Fatalf("shipped %s not found", RelPath)
	}
	for _, a := range m.Agents {
		if a.Provider != ProviderGitHubRelease {
			continue
		}
		// Every platform dots can run on must have a digest, or a sync
		// on that host fails at the last step instead of at review time.
		for _, target := range []string{"macos-aarch64", "macos-x86_64", "linux-aarch64", "linux-x86_64"} {
			if _, ok := a.SHA256[target]; !ok {
				t.Errorf("agents.%s: no sha256 for %s", a.Name, target)
			}
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
