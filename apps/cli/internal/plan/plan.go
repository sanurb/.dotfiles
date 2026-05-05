// Package plan owns the dots Plan type — the first-class artifact the
// CLI computes and consumes for every state-changing verb. A Plan
// describes exactly what `apply` would do, in order, with the exact
// commands. It is JSON-serializable, content-addressed by hash, and
// schema-versioned so a saved plan can be replayed against the same
// system state with confidence.
//
// Hash semantics: the hash covers schemaVersion, profile, host, and
// the ordered step list. It deliberately excludes generatedAt so two
// computations of the same plan over the same system match.
package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"
)

// SchemaVersion bumps when the Plan shape changes incompatibly.
// `apply --plan FILE` rejects mismatches loudly.
const SchemaVersion = 1

// Step.Kind values. Producers and consumers reference these constants
// rather than bare strings so a typo at any call site fails to compile
// instead of silently dropping to a default branch.
const (
	KindBootstrapNix      = "bootstrap-nix"
	KindCloneWorkspace    = "clone-workspace"
	KindSnapshotConflicts = "snapshot-conflicts"
	KindApplyProfile      = "apply-profile"
	KindInstallRuntimes   = "install-runtimes"
)

// Step.Action values — the diff verb. ActionAdd is "+", ActionChange is
// "~", ActionRemove is "-". Same rationale as the Kind constants.
const (
	ActionAdd    = "+"
	ActionChange = "~"
	ActionRemove = "-"
)

// Plan is the artifact a `dots plan` invocation emits and a `dots
// apply --plan FILE` invocation consumes. The JSON encoding is the
// stable wire format; the Go shape may grow private fields freely.
type Plan struct {
	SchemaVersion int       `json:"schemaVersion"`
	Hash          string    `json:"hash"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Profile       string    `json:"profile"`
	Host          Host      `json:"host"`
	Steps         []Step    `json:"steps"`
}

// Host is the machine fingerprint used to detect plan-vs-system drift.
type Host struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// Step is one ordered, executable unit of a Plan. Kind is the
// machine-stable identifier (`bootstrap-nix`, `clone-workspace`,
// `apply-profile`, `snapshot-conflicts`). Action is the diff verb
// ("+" add, "~" change, "-" remove). Command is the exact shell
// invocation the dots binary will run for this step (or empty if the
// step is in-process).
type Step struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Action  string   `json:"action"`
	Summary string   `json:"summary"`
	Command string   `json:"command,omitempty"`
	Effects []string `json:"effects,omitempty"`
}

// New returns a freshly populated Plan with the host fingerprint set.
// Caller is expected to append Steps and then call Seal() before
// emitting.
func New(profile string) Plan {
	return Plan{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Profile:       profile,
		Host:          CurrentHost(),
	}
}

// Seal computes and stamps the Plan's content hash. Call after all
// steps are appended and before serialization.
func (p *Plan) Seal() {
	p.Hash = p.ComputeHash()
}

// ComputeHash returns the canonical SHA-256 hex of the Plan's
// content-bearing fields. Excludes Hash and GeneratedAt so the same
// logical plan computed twice produces the same hash.
//
// Use Hash for plan-file integrity (`dots apply --plan FILE` requires
// the saved plan to match a fresh recomputation byte-for-byte). For
// drift detection and the already-applied no-op check, use
// ConvergedHash — that one survives prerequisite steps (snapshot,
// bootstrap, clone) appearing or disappearing across runs.
func (p Plan) ComputeHash() string {
	type canon struct {
		SchemaVersion int    `json:"schemaVersion"`
		Profile       string `json:"profile"`
		Host          Host   `json:"host"`
		Steps         []Step `json:"steps"`
	}
	b, _ := json.Marshal(canon{p.SchemaVersion, p.Profile, p.Host, p.Steps})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ConvergedHash returns the hash of the *desired converged state*
// described by this plan: profile, host, and the activation command
// the apply-profile step would run. Stable across plans that differ
// only in prerequisite work — bootstrap-nix, clone-workspace, and
// snapshot-conflicts steps don't affect it, since those represent
// "make the host ready," not "what state should the host be in."
//
// Used by:
//   - applied.toml: `dots apply` records this so the receipt survives
//     prerequisites being completed-and-then-gone.
//   - dots status: drift comparison answers "is the current state the
//     desired one?" rather than "is the work to get there identical?"
//   - isAlreadyApplied no-op short-circuit, same reason.
func (p Plan) ConvergedHash() string {
	type canon struct {
		SchemaVersion int    `json:"schemaVersion"`
		Profile       string `json:"profile"`
		Host          Host   `json:"host"`
		Command       string `json:"command"`
	}
	var cmd string
	for _, s := range p.Steps {
		if s.Kind == KindApplyProfile {
			cmd = s.Command
			break
		}
	}
	b, _ := json.Marshal(canon{p.SchemaVersion, p.Profile, p.Host, cmd})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// IsNoOp is true when the plan has no executable steps. Callers map
// this to exit-code NoOp under --dry-run.
func (p Plan) IsNoOp() bool { return len(p.Steps) == 0 }

// HasKind reports whether the plan contains at least one step whose
// Kind matches any of kinds. Used by callers that branch on "does
// this plan include a particular class of work" — e.g., consent
// gating around bootstrap-mutating steps, or "is there an
// apply-profile step to print."
func (p Plan) HasKind(kinds ...string) bool {
	for _, s := range p.Steps {
		for _, k := range kinds {
			if s.Kind == k {
				return true
			}
		}
	}
	return false
}

// Encode writes the Plan to w as indented JSON. The trailing newline
// is part of the contract — `dots plan > file.json` produces a file
// most editors will treat as well-formed.
func (p Plan) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}

// Decode reads a Plan from r and rejects schema-version mismatches.
func Decode(r io.Reader) (Plan, error) {
	var p Plan
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("decode plan: %w", err)
	}
	if p.SchemaVersion != SchemaVersion {
		return Plan{}, fmt.Errorf("plan schema mismatch: file is v%d, this dots speaks v%d", p.SchemaVersion, SchemaVersion)
	}
	return p, nil
}

// Counts returns (added, changed, removed) over Steps. Used by status
// and summary surfaces.
func (p Plan) Counts() (add, change, remove int) {
	for _, s := range p.Steps {
		switch s.Action {
		case ActionAdd:
			add++
		case ActionChange:
			change++
		case ActionRemove:
			remove++
		}
	}
	return
}

// CurrentHost returns the host fingerprint used by `dots plan` and
// `dots capture`. Exported so the capture verb shares one definition
// with the plan producer.
func CurrentHost() Host {
	name, _ := osHostname()
	return Host{Hostname: name, OS: runtime.GOOS, Arch: runtime.GOARCH}
}
