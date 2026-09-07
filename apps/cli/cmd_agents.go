package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/sanurb/.dotfiles/apps/cli/internal/activation"
	"github.com/sanurb/.dotfiles/apps/cli/internal/agents"
	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/envelope"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

const cmdAgentsSummary = "Report and converge agent CLI versions against config/agents/agents.toml"

// agentJSON is the per-row projection of agents.Status. Declared is
// empty for native agents, which pin nothing by design; consumers
// branch on state rather than on the presence of a version string.
type agentJSON struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Declared string `json:"declared,omitempty"`
	Observed string `json:"observed,omitempty"`
	State    string `json:"state"`
	Error    string `json:"error,omitempty"`
}

// agentsSummaryJSON is the roster tally. Agents branch on `drifted` and
// `missing` to decide whether a sync is worth running.
type agentsSummaryJSON struct {
	Total     int `json:"total"`
	Converged int `json:"converged"`
	Drifted   int `json:"drifted"`
	Missing   int `json:"missing"`
	Unmanaged int `json:"unmanaged"`
	Unknown   int `json:"unknown"`
}

type agentsStatusDocJSON struct {
	Manifest string            `json:"manifest"`
	Agents   []agentJSON       `json:"agents"`
	Summary  agentsSummaryJSON `json:"summary"`
}

// agentsSyncDocJSON reports what a sync did. DryRun distinguishes a
// plan from a realization, so an agent that pipes `--dry-run --json`
// into a decision never mistakes one for the other.
type agentsSyncDocJSON struct {
	Manifest string      `json:"manifest"`
	DryRun   bool        `json:"dryRun"`
	Planned  []agentJSON `json:"planned"`
	Synced   []agentJSON `json:"synced"`
	Skipped  []agentJSON `json:"skipped"`
	Failed   []agentJSON `json:"failed"`
}

// runAgents implements `dots agents [status|sync] [name...]`.
//
// The verb owns the agent-CLI plane: binaries whose release cadence is
// daily, which therefore live in neither flake.lock nor .prototools.
// It performs no Nix evaluation — that is the whole reason the plane
// is separate, and why syncing an agent costs milliseconds rather than
// a home-manager rebuild.
func runAgents(rest []string) int {
	// Peel the subcommand before flag parsing: the stdlib flag package
	// stops at the first non-flag argument, so `dots agents sync --json`
	// would otherwise drop --json on the floor.
	sub := "status"
	args := rest
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}

	fs := flag.NewFlagSet("agents "+sub, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var common cliflags.Common
	common.Bind(fs)
	if code, exit := cliflags.MapParseErr(fs.Parse(args)); exit {
		return code
	}
	common.Resolve()

	switch sub {
	case "status", "sync":
	default:
		if common.JSON {
			_ = envelope.Fail(os.Stdout, commandLine("agents", rest),
				envelope.New(envelope.CodeInvalidArgument,
					fmt.Sprintf("unknown subcommand %q (want: status, sync)", sub)))
			return exitcode.Misuse
		}
		fmt.Fprintf(os.Stderr, "agents: unknown subcommand %q (want: status, sync)\n", sub)
		return exitcode.Misuse
	}

	root, err := workspace.Root()
	if err != nil {
		// The dispatcher's requiresWorkspace gate already covers this;
		// the branch remains so a direct call is not a nil-root panic.
		if common.JSON {
			_ = envelope.Fail(os.Stdout, commandLine("agents", rest),
				envelope.Wrap(envelope.CodeWorkspaceNotFound, err))
			return exitcode.Misuse
		}
		fmt.Fprintln(os.Stderr, "agents:", err)
		return exitcode.Misuse
	}

	path := agents.Path(root)
	manifest, found, err := agents.Load(path)
	if err != nil {
		if common.JSON {
			_ = envelope.Fail(os.Stdout, commandLine("agents", rest),
				envelope.Wrap(envelope.CodeAgentManifestInvalid, err))
			return exitcode.PreFlight
		}
		fmt.Fprintln(os.Stderr, "agents: manifest is invalid:")
		fmt.Fprintf(os.Stderr, "  what: %s\n", agents.RelPath)
		fmt.Fprintf(os.Stderr, "  why:  %s\n", err)
		fmt.Fprintln(os.Stderr, "  next: fix the reported row, then re-run `dots agents status`")
		return exitcode.PreFlight
	}
	if !found {
		// No BOM is a legitimate state for a host that has not adopted
		// the plane. Informational, not an error — same policy as
		// `dots status` outside a workspace.
		if common.JSON {
			_ = envelope.OK(os.Stdout, commandLine("agents", rest),
				agentsStatusDocJSON{Manifest: path}, nil)
		} else {
			fmt.Fprintf(os.Stderr, "agents: no %s; nothing declared\n", agents.RelPath)
		}
		return exitcode.Success
	}

	statuses := agents.Inspect(context.Background(), manifest, nil)
	if selected := fs.Args(); len(selected) > 0 {
		filtered, unknown := selectAgents(statuses, selected)
		if len(unknown) > 0 {
			msg := fmt.Sprintf("unknown agent(s): %s (declared: %s)",
				strings.Join(unknown, ", "), strings.Join(manifest.Names(), ", "))
			if common.JSON {
				_ = envelope.Fail(os.Stdout, commandLine("agents", rest),
					envelope.New(envelope.CodeInvalidArgument, msg))
				return exitcode.Misuse
			}
			fmt.Fprintln(os.Stderr, "agents:", msg)
			return exitcode.Misuse
		}
		statuses = filtered
	}

	if sub == "status" {
		return runAgentsStatus(rest, path, statuses, common)
	}
	return runAgentsSync(rest, path, statuses, common)
}

// selectAgents narrows the roster to the named rows, preserving the
// manifest's sorted order. Unknown names are returned rather than
// ignored: silently skipping a typo'd name would report success for an
// agent that was never touched.
func selectAgents(statuses []agents.Status, names []string) (selected []agents.Status, unknown []string) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	for _, s := range statuses {
		if want[s.Agent.Name] {
			selected = append(selected, s)
			delete(want, s.Agent.Name)
		}
	}
	for _, n := range names {
		if want[n] {
			unknown = append(unknown, n)
		}
	}
	return selected, unknown
}

// runAgentsStatus is read-only and always exits 0 on a readable
// manifest. Drift is reported, not escalated: a caller that wants drift
// to be fatal branches on `.summary.drifted` in the JSON body.
func runAgentsStatus(rest []string, path string, statuses []agents.Status, common cliflags.Common) int {
	if common.JSON {
		body := agentsStatusDocJSON{
			Manifest: path,
			Agents:   agentsJSON(statuses),
			Summary:  summarize(statuses),
		}
		_ = envelope.OK(os.Stdout, commandLine("agents", rest), body, agentsStatusActions(statuses))
		return exitcode.Success
	}
	renderAgentsHuman(statuses)
	return exitcode.Success
}

// runAgentsSync converges every drifted or missing managed agent.
// Native agents are never touched — they ship their own updater, and
// the BOM records that as a decision rather than an omission.
//
// One agent's failure does not abort the rest: the loop collects
// failures and reports them together, so a checksum mismatch on herdr
// still lets pi converge.
func runAgentsSync(rest []string, path string, statuses []agents.Status, common cliflags.Common) int {
	var planned []agents.Status
	for _, s := range statuses {
		if s.NeedsSync() {
			planned = append(planned, s)
		}
	}

	if len(planned) == 0 {
		if common.JSON {
			body := agentsSyncDocJSON{Manifest: path, DryRun: common.DryRun}
			_ = envelope.OK(os.Stdout, commandLine("agents", rest), body, nil)
			return exitcode.NoOp
		}
		fmt.Fprintln(os.Stderr, "agents: already converged; nothing to do")
		return exitcode.NoOp
	}

	if common.DryRun {
		if common.JSON {
			body := agentsSyncDocJSON{Manifest: path, DryRun: true, Planned: agentsJSON(planned)}
			_ = envelope.OK(os.Stdout, commandLine("agents", rest), body, nil)
			return exitcode.Success
		}
		for _, s := range planned {
			fmt.Printf("would install %s %s (%s)%s\n",
				s.Agent.Name, s.Agent.Version, s.Agent.Provider, replacing(s))
			if url, err := agentSource(s.Agent); err == nil {
				fmt.Printf("    from %s\n", url)
			}
		}
		return exitcode.Success
	}

	notify := func(format string, args ...any) {
		if !common.Quiet {
			fmt.Fprintf(os.Stderr, "agents: "+format+"\n", args...)
		}
	}
	out := convergeAgents(context.Background(), planned, nil, notify)

	if common.JSON {
		if len(out.Failed) > 0 {
			_ = envelope.Fail(os.Stdout, commandLine("agents", rest),
				envelope.Wrap(envelope.CodeAgentSyncFailed, joinAgentErrors(out.Failed)))
			return exitcode.Failure
		}
		body := agentsSyncDocJSON{
			Manifest: path,
			Synced:   agentsJSON(out.Synced),
			Skipped:  agentsJSON(out.Skipped),
		}
		_ = envelope.OK(os.Stdout, commandLine("agents", rest), body, nil)
		return exitcode.Success
	}

	for _, s := range out.Synced {
		fmt.Fprintf(os.Stderr, "agents: %s is now %s\n", s.Agent.Name, s.Observed)
	}
	if len(out.Failed) > 0 {
		for _, s := range out.Failed {
			fmt.Fprintf(os.Stderr, "agents: %s failed: %v\n", s.Agent.Name, s.Err)
		}
		return exitcode.Failure
	}
	return exitcode.Success
}

// agentSyncOutcome buckets one converge pass. Skipped is deliberately
// distinct from Failed: it holds only rows whose provider toolchain is
// not on PATH yet, which on a fresh host is an ordering artifact of the
// very apply that installs that toolchain, not a defect.
type agentSyncOutcome struct {
	Synced  []agents.Status
	Skipped []agents.Status
	Failed  []agents.Status
}

// convergeAgents installs each planned row and re-probes what landed.
// Shared by the `dots agents sync` verb and the sync-agents plan step
// so both classify outcomes identically.
//
// One row's failure never aborts the rest: a stale digest on herdr must
// still let pi converge, and the caller reports the whole picture.
func convergeAgents(
	ctx context.Context,
	planned []agents.Status,
	resolve agents.Resolver,
	notify func(format string, args ...any),
) agentSyncOutcome {
	var out agentSyncOutcome
	for _, s := range planned {
		notify("installing %s %s%s", s.Agent.Name, s.Agent.Version, replacing(s))
		if err := agents.Converge(ctx, s.Agent, resolve); err != nil {
			s.Err = err
			if errors.Is(err, agents.ErrToolchainMissing) {
				out.Skipped = append(out.Skipped, s)
				notify("skipped %s: %v", s.Agent.Name, err)
				continue
			}
			out.Failed = append(out.Failed, s)
			continue
		}
		// Re-probe so the report states what the host now has rather
		// than what the BOM asked for. An installer that lands a
		// different version than requested is exactly the failure this
		// plane exists to catch.
		out.Synced = append(out.Synced, agents.Inspect(ctx, agents.Manifest{
			SchemaVersion: agents.SchemaVersion,
			Agents:        []agents.Agent{s.Agent},
		}, resolve)[0])
	}
	return out
}

// syncAgentsStep executes the sync-agents plan step for `dots apply`.
// Returns nil when the plane converged (including the nothing-to-do and
// graceful-skip cases) and a Problem when a declared agent could not be
// installed for a reason the user has to act on.
//
// The resolver is bound to the activation env rather than to this
// process's PATH so the step probes and installs against exactly the
// PATH the rest of apply hands to nh — internal/activation's package
// comment makes reconstructing that env anywhere else a regression.
//
// Progress goes to w: os.Stderr on the prose path, the per-run log file
// on the streaming path.
func syncAgentsStep(ctx context.Context, env []string, w io.Writer) *envelope.Problem {
	notify := func(format string, args ...any) {
		fmt.Fprintf(w, "sync-agents: "+format+"\n", args...)
	}

	root, err := workspace.Root()
	if err != nil {
		notify("workspace not resolved; skipping")
		return nil
	}
	manifest, found, err := agents.Load(agents.Path(root))
	if err != nil {
		return envelope.Wrap(envelope.CodeAgentManifestInvalid, err)
	}
	if !found {
		// computePlan stat'd the file, so this is a race (or a
		// hand-deleted manifest between plan and apply). Nothing
		// declared means nothing to converge.
		notify("no %s; nothing declared", agents.RelPath)
		return nil
	}

	resolve := agents.Resolver(func(name string) (string, error) {
		return activation.LookPathIn(name, env)
	})

	var planned []agents.Status
	for _, s := range agents.Inspect(ctx, manifest, resolve) {
		if s.NeedsSync() {
			planned = append(planned, s)
		}
	}
	if len(planned) == 0 {
		notify("already converged (%d agent(s) declared)", len(manifest.Agents))
		return nil
	}

	out := convergeAgents(ctx, planned, resolve, notify)
	for _, s := range out.Synced {
		notify("%s is now %s", s.Agent.Name, s.Observed)
	}
	if len(out.Failed) > 0 {
		for _, s := range out.Failed {
			notify("%s failed: %v", s.Agent.Name, s.Err)
		}
		return envelope.Wrap(envelope.CodeAgentSyncFailed, joinAgentErrors(out.Failed)).
			WithFix("Fix the reported agent(s) in " + agents.RelPath +
				", or re-run `dots apply --skip-agents` to converge the Nix plane without them.")
	}
	return nil
}

// replacing renders the " (was X)" suffix for a drifted agent so the
// install line says what it is displacing. Missing agents get nothing.
func replacing(s agents.Status) string {
	if s.Observed == "" {
		return ""
	}
	return " (was " + s.Observed + ")"
}

// agentSource renders the provider-specific origin for --dry-run.
// Only github-release has a URL worth showing; bun's origin is the
// registry and is implied by the package specifier.
func agentSource(a agents.Agent) (string, error) {
	if a.Provider != agents.ProviderGitHubRelease {
		return "", fmt.Errorf("no URL for provider %q", a.Provider)
	}
	target, err := agents.Target()
	if err != nil {
		return "", err
	}
	return agents.AssetURL(a, target)
}

func agentsJSON(statuses []agents.Status) []agentJSON {
	out := make([]agentJSON, 0, len(statuses))
	for _, s := range statuses {
		row := agentJSON{
			Name:     s.Agent.Name,
			Provider: string(s.Agent.Provider),
			Declared: s.Declared,
			Observed: s.Observed,
			State:    s.State.String(),
		}
		if s.Err != nil {
			row.Error = s.Err.Error()
		}
		out = append(out, row)
	}
	return out
}

func summarize(statuses []agents.Status) agentsSummaryJSON {
	sum := agentsSummaryJSON{Total: len(statuses)}
	for _, s := range statuses {
		switch s.State {
		case agents.StateConverged:
			sum.Converged++
		case agents.StateDrifted:
			sum.Drifted++
		case agents.StateMissing:
			sum.Missing++
		case agents.StateUnmanaged, agents.StateUnmanagedMissing:
			sum.Unmanaged++
		default:
			sum.Unknown++
		}
	}
	return sum
}

func joinAgentErrors(failed []agents.Status) error {
	parts := make([]string, 0, len(failed))
	for _, s := range failed {
		parts = append(parts, fmt.Sprintf("%s: %v", s.Agent.Name, s.Err))
	}
	return fmt.Errorf("%s", strings.Join(parts, "; "))
}

// agentsStatusActions makes `dots agents sync` the primary affordance
// whenever the roster has drifted or missing rows, and drops to the
// broader status verb once the plane is converged.
func agentsStatusActions(statuses []agents.Status) []envelope.Action {
	for _, s := range statuses {
		if s.NeedsSync() {
			return []envelope.Action{
				{Command: "dots agents sync", Description: "Install every declared agent at its pinned version."},
				{Command: "dots agents sync --dry-run", Description: "Show what sync would install, without installing."},
			}
		}
	}
	return []envelope.Action{
		{Command: "dots status", Description: "Report the Nix-managed plane's drift."},
	}
}

// renderAgentsHuman prints the roster as an aligned table on stdout.
// Probe errors go to stderr so `dots agents | grep` never picks up
// diagnostic noise — same split as renderStatusHuman.
func renderAgentsHuman(statuses []agents.Status) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROVIDER\tDECLARED\tOBSERVED\tSTATE")
	for _, s := range statuses {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			s.Agent.Name, s.Agent.Provider,
			dashIfEmpty(s.Declared), dashIfEmpty(s.Observed), s.State)
	}
	_ = w.Flush()

	for _, s := range statuses {
		if s.Err != nil {
			fmt.Fprintf(os.Stderr, "agents: %s: %v\n", s.Agent.Name, s.Err)
		}
	}
	sum := summarize(statuses)
	if sum.Drifted+sum.Missing > 0 {
		fmt.Fprintf(os.Stderr, "\nagents: %d drifted, %d missing — run `dots agents sync`\n",
			sum.Drifted, sum.Missing)
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
