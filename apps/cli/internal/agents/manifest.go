// Package agents owns config/agents/agents.toml — the Bill of Materials
// for agent CLIs whose release cadence is too fast for flake.lock and
// too fast for .prototools.
//
// The plane exists because agent binaries were previously installed by
// Home Manager activation hooks that guarded on binary existence, not
// version: once present, they were never upgraded and their version was
// recorded nowhere. Here the declared version is the guard, so a host
// converges to the BOM instead of to whatever it happened to install
// first.
//
// TOML is hand-rolled for the same reason internal/state hand-rolls it:
// the schema is closed and small, and a ~130-line parser is preferable
// to a dependency the doctor would then also have to verify. The one
// shape this parser adds over state's is a nested table
// ([agents.<name>.sha256]), which arrives as a dotted section path.
package agents

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// RelPath is the BOM's location under the workspace root. Exported so
// the doctor and any future Nix-side reader name the same path.
const RelPath = "config/agents/agents.toml"

// SchemaVersion bumps when the file shape changes incompatibly. Unlike
// .dots-state.toml this file is hand-edited, so a mismatch is reported
// rather than silently migrated.
const SchemaVersion = 1

// Provider selects the install mechanism. The set is closed: an unknown
// provider is a validation error, never a silent skip, because silently
// skipping an agent is how the old activation hooks went stale.
type Provider string

const (
	// ProviderGitHubRelease downloads a release asset over HTTPS and
	// verifies it against the SHA-256 recorded in the BOM before
	// installing. The only provider that gives a reproducibility
	// guarantee comparable to a Nix fetchurl.
	ProviderGitHubRelease Provider = "github-release"

	// ProviderBun installs an npm package through bun's global prefix
	// (~/.bun/bin), pinned with an exact `pkg@version` specifier.
	ProviderBun Provider = "bun"

	// ProviderNative marks an agent that ships its own autoupdater.
	// dots observes and reports its version but never converges it.
	// Declaring a version on a native agent is rejected: it would be a
	// claim the reconciler does not enforce.
	ProviderNative Provider = "native"
)

// Agent is one BOM row.
type Agent struct {
	Name     string
	Provider Provider
	Version  string // required except for ProviderNative, which forbids it
	Probe    string // argv rendered with spaces, e.g. "herdr --version"

	// ProviderGitHubRelease
	Repo   string            // "owner/name"
	Asset  string            // asset name template over {os} and {arch}
	SHA256 map[string]string // keyed "<os>-<arch>", see platform.go

	// ProviderBun
	Package string
}

// Managed reports whether the reconciler may install this agent. A
// native agent is observable but never converged.
func (a Agent) Managed() bool { return a.Provider != ProviderNative }

// Manifest is the parsed BOM. Agents is sorted by name so every
// surface (status table, JSON body, sync order) is deterministic
// regardless of the order rows appear in the file.
type Manifest struct {
	SchemaVersion int
	Agents        []Agent
}

// Lookup returns the named agent. Used by `dots agents sync <name>`.
func (m Manifest) Lookup(name string) (Agent, bool) {
	for _, a := range m.Agents {
		if a.Name == name {
			return a, true
		}
	}
	return Agent{}, false
}

// Names returns every declared agent name in sorted order.
func (m Manifest) Names() []string {
	out := make([]string, len(m.Agents))
	for i, a := range m.Agents {
		out[i] = a.Name
	}
	return out
}

// Path returns the absolute BOM path for a workspace root.
func Path(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, RelPath)
}

// Load reads and validates the BOM. A missing file is not an error:
// found=false lets callers report "no agents declared" rather than
// failing a host that has not adopted the plane yet.
func Load(path string) (m Manifest, found bool, err error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return Manifest{SchemaVersion: SchemaVersion}, false, nil
	}
	if err != nil {
		return Manifest{}, false, err
	}
	defer f.Close()

	parsed, perr := parse(f)
	if perr != nil {
		return Manifest{}, false, fmt.Errorf("parse %s: %w", path, perr)
	}
	if verr := parsed.Validate(); verr != nil {
		return Manifest{}, false, fmt.Errorf("validate %s: %w", path, verr)
	}
	return parsed, true, nil
}

// Validate enforces the closed provider set and each provider's
// required fields. Every failure names the offending row so a
// hand-edit error points at its own line rather than at the parser.
func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %d is not supported (this build speaks %d)",
			m.SchemaVersion, SchemaVersion)
	}
	if len(m.Agents) == 0 {
		return fmt.Errorf("no [agents.<name>] tables declared")
	}
	for _, a := range m.Agents {
		if err := a.validate(); err != nil {
			return fmt.Errorf("agents.%s: %w", a.Name, err)
		}
	}
	return nil
}

func (a Agent) validate() error {
	if a.Probe == "" {
		return fmt.Errorf("probe is required (the command that prints the installed version)")
	}
	switch a.Provider {
	case ProviderGitHubRelease:
		if a.Version == "" {
			return fmt.Errorf("version is required")
		}
		if a.Repo == "" || !strings.Contains(a.Repo, "/") {
			return fmt.Errorf("repo must be \"owner/name\", got %q", a.Repo)
		}
		if a.Asset == "" {
			return fmt.Errorf("asset is required")
		}
		if len(a.SHA256) == 0 {
			return fmt.Errorf("[agents.%s.sha256] must declare at least one platform digest", a.Name)
		}
		for target, digest := range a.SHA256 {
			if len(digest) != 64 || strings.ContainsFunc(digest, notHexDigit) {
				return fmt.Errorf("sha256.%s is not a 64-character hex digest", target)
			}
		}
	case ProviderBun:
		if a.Version == "" {
			return fmt.Errorf("version is required")
		}
		if a.Package == "" {
			return fmt.Errorf("package is required")
		}
	case ProviderNative:
		// A version here would be a claim nothing enforces. Reject it
		// so the file never implies a guarantee the reconciler skips.
		if a.Version != "" {
			return fmt.Errorf("provider \"native\" self-updates; remove the version (it would never be enforced)")
		}
	case "":
		return fmt.Errorf("provider is required (one of %s)", strings.Join(providerNames(), ", "))
	default:
		return fmt.Errorf("unknown provider %q (allowed: %s)", a.Provider, strings.Join(providerNames(), ", "))
	}
	return nil
}

func providerNames() []string {
	return []string{string(ProviderBun), string(ProviderGitHubRelease), string(ProviderNative)}
}

func notHexDigit(r rune) bool {
	return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F')
}

// parse handles the closed schema:
//
//	schema_version = 1
//	[agents.<name>]
//	provider = "github-release" | "bun" | "native"
//	version  = "0.8.2"
//	probe    = "herdr --version"
//	repo     = "herdrdev/herdr"      # github-release
//	asset    = "herdr-{os}-{arch}"   # github-release
//	package  = "@scope/pkg"          # bun
//	[agents.<name>.sha256]
//	<os>-<arch> = "<64 hex chars>"
//
// Unlike internal/state, an unrecognized key is an error rather than a
// silent drop. state's file is machine-written and a stray key there is
// noise; this file is hand-edited, and silently ignoring a typo'd
// `verison` would leave the agent pinned to nothing while the file
// reads as if it were pinned.
func parse(r io.Reader) (Manifest, error) {
	out := Manifest{}
	byName := map[string]*Agent{}

	sc := bufio.NewScanner(r)
	line := 0
	section := ""
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
			section = strings.TrimSpace(text[1 : len(text)-1])
			// Registering on the header (not on the first key) means an
			// agent declared with no keys still reaches Validate and
			// fails there with a useful message.
			if name, sub, err := splitAgentSection(section); err != nil {
				return Manifest{}, fmt.Errorf("line %d: %w", line, err)
			} else if sub == "" {
				if _, dup := byName[name]; dup {
					return Manifest{}, fmt.Errorf("line %d: agent %q declared twice", line, name)
				}
				byName[name] = &Agent{Name: name}
			}
			continue
		}

		eq := strings.Index(text, "=")
		if eq < 0 {
			return Manifest{}, fmt.Errorf("line %d: expected `key = value`, got %q", line, text)
		}
		key := strings.TrimSpace(text[:eq])
		raw := strings.TrimSpace(text[eq+1:])
		if i := strings.Index(raw, " #"); i >= 0 {
			raw = strings.TrimSpace(raw[:i])
		}

		if section == "" {
			if key != "schema_version" {
				return Manifest{}, fmt.Errorf("line %d: unknown top-level key %q", line, key)
			}
			n, err := strconv.Atoi(raw)
			if err != nil {
				return Manifest{}, fmt.Errorf("line %d: schema_version must be an integer, got %q", line, raw)
			}
			out.SchemaVersion = n
			continue
		}

		name, sub, err := splitAgentSection(section)
		if err != nil {
			return Manifest{}, fmt.Errorf("line %d: %w", line, err)
		}
		agent, ok := byName[name]
		if !ok {
			return Manifest{}, fmt.Errorf("line %d: [agents.%s.%s] has no [agents.%s] table", line, name, sub, name)
		}
		val, ok := unquote(raw)
		if !ok {
			return Manifest{}, fmt.Errorf("line %d: %s.%s: expected a quoted string, got %q", line, section, key, raw)
		}

		if sub == "sha256" {
			if agent.SHA256 == nil {
				agent.SHA256 = map[string]string{}
			}
			agent.SHA256[key] = val
			continue
		}
		switch key {
		case "provider":
			agent.Provider = Provider(val)
		case "version":
			agent.Version = val
		case "probe":
			agent.Probe = val
		case "repo":
			agent.Repo = val
		case "asset":
			agent.Asset = val
		case "package":
			agent.Package = val
		default:
			return Manifest{}, fmt.Errorf("line %d: agents.%s: unknown key %q", line, name, key)
		}
	}
	if err := sc.Err(); err != nil {
		return Manifest{}, err
	}

	out.Agents = make([]Agent, 0, len(byName))
	for _, a := range byName {
		out.Agents = append(out.Agents, *a)
	}
	sort.Slice(out.Agents, func(i, j int) bool { return out.Agents[i].Name < out.Agents[j].Name })
	return out, nil
}

// splitAgentSection decomposes "agents.<name>[.<sub>]" into its parts.
// Any other section header is rejected outright — the BOM has exactly
// one top-level table, and a stray [tools.x] would otherwise parse into
// nothing and read as if it were honored.
func splitAgentSection(section string) (name, sub string, err error) {
	parts := strings.Split(section, ".")
	if len(parts) < 2 || parts[0] != "agents" {
		return "", "", fmt.Errorf("unknown section [%s]; expected [agents.<name>] or [agents.<name>.sha256]", section)
	}
	switch len(parts) {
	case 2:
		return parts[1], "", nil
	case 3:
		if parts[2] != "sha256" {
			return "", "", fmt.Errorf("unknown sub-table [%s]; only .sha256 is defined", section)
		}
		return parts[1], parts[2], nil
	default:
		return "", "", fmt.Errorf("section [%s] is nested too deeply", section)
	}
}

func unquote(v string) (string, bool) {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1], true
	}
	return "", false
}
