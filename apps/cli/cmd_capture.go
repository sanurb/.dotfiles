package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

const cmdCaptureSummary = "Extract host metadata and current profile to TOML/JSON"

// captureProfile mirrors state.State for the JSON encoder. The state
// package's own struct fields are not JSON-tagged (it speaks TOML on
// disk only), so we project into a small JSON-friendly view here.
type captureProfile struct {
	Shell       string `json:"shell"`
	Terminal    string `json:"terminal"`
	Multiplexer string `json:"multiplexer"`
	Editor      bool   `json:"editor"`
	Git         bool   `json:"git"`
}

// captureDoc is the full envelope serialized to the user. profile is a
// pointer so JSON emits null (not a zero struct) when no state file is
// reachable; the human/TOML renderer treats nil as "[profile] omitted".
type captureDoc struct {
	SchemaVersion int             `json:"schemaVersion"`
	Host          plan.Host       `json:"host"`
	Profile       *captureProfile `json:"profile"`
}

// runCapture implements `dots capture [--output PATH] [--json]`. Default
// format is TOML (matching `.dots-state.toml`'s shape with an added
// [host] table); --json emits the JSON envelope above. Default sink is
// stdout; --output FILE writes to FILE, creating parent dirs.
func runCapture(rest []string) int {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	var common cliflags.Common
	common.Bind(fs)
	output := fs.String("output", "", "write captured document to PATH (default: stdout)")
	if code, exit := cliflags.MapParseErr(fs.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	doc := buildCaptureDoc()

	body, err := renderCapture(doc, common.JSON)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capture:", err)
		return exitcode.Failure
	}

	if *output == "" {
		_, _ = os.Stdout.Write(body)
		return exitcode.Success
	}

	if err := writeCaptureFile(*output, body); err != nil {
		fmt.Fprintf(os.Stderr, "capture: write %s: %v\n", *output, err)
		fmt.Fprintln(os.Stderr, "next: confirm the parent directory exists and is writable, or pick a different --output path")
		return exitcode.Failure
	}
	return exitcode.Success
}

// buildCaptureDoc collects the host fingerprint and (optionally) the
// profile from the workspace's `.dots-state.toml`. A missing or
// unparseable state file is NOT an error — the doc is still useful for
// "I want to seed a new machine" — it just renders without [profile].
//
// We try workspace.Root() first; if that fails (running outside a
// clone) we look for `.dots-state.toml` in the current working
// directory as a courtesy for users running capture from a checkout
// they staged manually.
func buildCaptureDoc() captureDoc {
	doc := captureDoc{
		SchemaVersion: state.SchemaVersion,
		Host:          plan.CurrentHost(),
	}

	candidates := captureStatePaths()
	for _, p := range candidates {
		s, found, err := state.Load(p)
		if err != nil || !found {
			continue
		}
		doc.Profile = &captureProfile{
			Shell:       s.Pillars.Shell,
			Terminal:    s.Pillars.Terminal,
			Multiplexer: s.Pillars.Multiplexer,
			Editor:      s.Capabilities.Editor,
			Git:         s.Capabilities.Git,
		}
		// Trust the state file's schema version when present so a
		// captured doc round-trips faithfully through `dots install`
		// on a peer host.
		doc.SchemaVersion = s.SchemaVersion
		break
	}
	return doc
}

// captureStatePaths returns the ordered list of candidate paths to
// look for `.dots-state.toml`. Workspace root takes precedence; cwd is
// the fallback for the "manual checkout" flow described above.
func captureStatePaths() []string {
	var paths []string
	if root, err := workspace.Root(); err == nil {
		paths = append(paths, state.Path(root))
	}
	if wd, err := os.Getwd(); err == nil {
		p := filepath.Join(wd, state.FileName)
		// Avoid duplicate when the workspace root IS the cwd.
		if len(paths) == 0 || paths[0] != p {
			paths = append(paths, p)
		}
	}
	return paths
}

// renderCapture serializes the doc to bytes per the requested format.
// JSON uses encoding/json with two-space indent, matching the rest of
// the CLI's machine-readable output. TOML is hand-rolled to mirror the
// `state` package's emitter style (the state package keeps its emitter
// private, so we do not call into it).
func renderCapture(doc captureDoc, asJSON bool) ([]byte, error) {
	if asJSON {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(doc); err != nil {
			return nil, fmt.Errorf("encode JSON: %w", err)
		}
		return buf.Bytes(), nil
	}
	return renderCaptureTOML(doc), nil
}

// renderCaptureTOML emits a TOML document matching `.dots-state.toml`'s
// shape, plus a `[host]` table. The header comment names the verb so
// future readers know how to regenerate it.
func renderCaptureTOML(doc captureDoc) []byte {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "# captured by `dots capture` — host metadata + active profile.")
	fmt.Fprintln(&buf, "# Editable; intended as a portable artifact for seeding a new machine.")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "schema_version = %d\n", doc.SchemaVersion)
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "[host]")
	fmt.Fprintf(&buf, "hostname = %q\n", doc.Host.Hostname)
	fmt.Fprintf(&buf, "os       = %q\n", doc.Host.OS)
	fmt.Fprintf(&buf, "arch     = %q\n", doc.Host.Arch)
	if doc.Profile != nil {
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "[pillars]")
		fmt.Fprintf(&buf, "shell       = %q\n", doc.Profile.Shell)
		fmt.Fprintf(&buf, "terminal    = %q\n", doc.Profile.Terminal)
		fmt.Fprintf(&buf, "multiplexer = %q\n", doc.Profile.Multiplexer)
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "[capabilities]")
		fmt.Fprintf(&buf, "editor = %v\n", doc.Profile.Editor)
		fmt.Fprintf(&buf, "git    = %v\n", doc.Profile.Git)
	}
	return buf.Bytes()
}

// writeCaptureFile creates parent directories and atomically replaces
// the target. We use the same .tmp-then-rename dance the state package
// uses so a captured doc never exists on disk in a half-written state
// — readers (CI, peer hosts) can `cat` it without race-window concerns.
func writeCaptureFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
