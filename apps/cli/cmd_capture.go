package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/envelope"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/plan"
	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

const cmdCaptureSummary = "Extract host metadata and current profile to TOML/JSON"

// captureDoc is the full envelope serialized to the user. profile is a
// pointer so JSON emits null (not a zero struct) when no state file is
// reachable; the human/TOML renderer treats nil as "[profile] omitted".
type captureDoc struct {
	SchemaVersion int          `json:"schemaVersion"`
	Host          plan.Host    `json:"host"`
	Profile       *personaJSON `json:"profile"`
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

	// File path: the on-disk artifact is always the same shape
	// (TOML or raw-doc JSON) regardless of --json on stdout — peer
	// hosts and `dots install` consume it directly. The envelope is
	// only emitted on stdout, never written to --output.
	if *output != "" {
		body, err := renderCapture(doc, common.JSON)
		if err != nil {
			if common.JSON {
				_ = envelope.Fail(os.Stdout, commandLine("capture", rest),
					envelope.Wrap(envelope.CodeInternalError, err))
				return exitcode.Failure
			}
			fmt.Fprintln(os.Stderr, "capture:", err)
			return exitcode.Failure
		}
		if err := writeCaptureFile(*output, body); err != nil {
			if common.JSON {
				_ = envelope.Fail(os.Stdout, commandLine("capture", rest),
					envelope.Wrap(envelope.CodeInvalidArgument, fmt.Errorf("write %s: %w", *output, err)).
						WithFix("Confirm the parent directory exists and is writable, or pick a different --output path."))
				return exitcode.Failure
			}
			fmt.Fprintf(os.Stderr, "capture: write %s: %v\n", *output, err)
			fmt.Fprintln(os.Stderr, "next: confirm the parent directory exists and is writable, or pick a different --output path")
			return exitcode.Failure
		}
		if common.JSON {
			_ = envelope.OK(os.Stdout, commandLine("capture", rest),
				captureSummary(doc, *output),
				captureActionsWithFile(*output))
			return exitcode.Success
		}
		return exitcode.Success
	}

	// stdout sink:
	if common.JSON {
		_ = envelope.OK(os.Stdout, commandLine("capture", rest), doc, captureActions())
		return exitcode.Success
	}
	body, err := renderCapture(doc, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capture:", err)
		return exitcode.Failure
	}
	_, _ = os.Stdout.Write(body)
	return exitcode.Success
}

// captureSummaryBody is the result body when --output wrote the doc
// to a file. Pointing at the artifact rather than duplicating it on
// stdout keeps the envelope small and matches the plan-with-out
// pattern.
type captureSummaryBody struct {
	OutPath  string `json:"out_path"`
	Hostname string `json:"hostname"`
	Profile  string `json:"profile,omitempty"` // pillar one-liner when present
}

func captureSummary(doc captureDoc, outPath string) captureSummaryBody {
	body := captureSummaryBody{OutPath: outPath, Hostname: doc.Host.Hostname}
	if doc.Profile != nil {
		body.Profile = doc.Profile.Shell + " · " + doc.Profile.Terminal + " · " + doc.Profile.Multiplexer
	}
	return body
}

func captureActions() []envelope.Action {
	return []envelope.Action{
		{
			Command:     "dots capture --output <path>",
			Description: "Save the captured doc as an artifact for `dots init --config <path>`.",
			Params: map[string]envelope.ActionParam{
				"path": {Description: "File path to write the captured TOML or JSON.", Required: true},
			},
		},
	}
}

func captureActionsWithFile(outPath string) []envelope.Action {
	return []envelope.Action{
		{
			Command:     "dots init --non-interactive --config <path>",
			Description: "Seed the install wizard from this captured doc on a peer host.",
			Params: map[string]envelope.ActionParam{
				"path": {Description: "Path to the file just written.", Value: outPath, Required: true},
			},
		},
	}
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
		p := personaJSONFromState(s)
		doc.Profile = &p
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
		fmt.Fprintf(&buf, "font   = %v\n", doc.Profile.Font)
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
