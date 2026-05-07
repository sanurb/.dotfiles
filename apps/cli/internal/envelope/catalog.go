package envelope

import "slices"

// Code is the SCREAMING_SNAKE error identifier emitted as
// .error.code in every error envelope. The set is closed: every
// possible error path in the dots CLI maps to exactly one Code.
// Adding a Code requires registering it in the catalog below;
// TestCatalogIsComplete fails the build otherwise.
type Code string

const (
	// CodeWorkspaceNotFound — the verb requires a cloned dotfiles
	// workspace and none was found at or above cwd.
	CodeWorkspaceNotFound Code = "WORKSPACE_NOT_FOUND"

	// CodeConfigNotFound — --config PATH was given and PATH does not
	// exist or is unreadable.
	CodeConfigNotFound Code = "CONFIG_NOT_FOUND"

	// CodeConfigInvalid — --config PATH parsed but failed validation
	// (e.g., shell value not in the closed pillar set).
	CodeConfigInvalid Code = "CONFIG_INVALID"

	// CodeStateParseFailed — .dots-state.toml exists but is
	// syntactically malformed.
	CodeStateParseFailed Code = "STATE_PARSE_FAILED"

	// CodeStateInvalid — .dots-state.toml parsed but a pillar value
	// is outside its closed set.
	CodeStateInvalid Code = "STATE_INVALID"

	// CodeBootstrapRequired — Nix or the workspace clone is missing
	// and the verb cannot proceed without interactive consent.
	CodeBootstrapRequired Code = "BOOTSTRAP_REQUIRED"

	// CodeBuildFailed — the underlying Nix evaluation or derivation
	// build failed (the work nh delegates to).
	CodeBuildFailed Code = "BUILD_FAILED"

	// CodeActivationFailed — the build succeeded but the
	// home-manager activation step failed (the post-build hook).
	CodeActivationFailed Code = "ACTIVATION_FAILED"

	// CodePreflightFailed — `dots doctor` reported a SevFail before
	// activation; apply refused to proceed.
	CodePreflightFailed Code = "PREFLIGHT_FAILED"

	// CodePlanStale — `dots apply --plan FILE` was given a saved
	// plan whose hash no longer matches a freshly-computed plan.
	CodePlanStale Code = "PLAN_STALE"

	// CodeDeclined — the user declined a confirmation prompt. Not a
	// retryable failure; semantically a user-initiated terminal
	// decision.
	CodeDeclined Code = "DECLINED"

	// CodeAborted — wizard aborted (Ctrl-C, Esc on welcome, or
	// Cancel on a binary prompt).
	CodeAborted Code = "ABORTED"

	// CodeUnknownCommand — the dispatcher received a verb (or alias)
	// that resolves to nothing.
	CodeUnknownCommand Code = "UNKNOWN_COMMAND"

	// CodeInvalidArgument — a flag value or positional argument is
	// outside the verb's accepted shape.
	CodeInvalidArgument Code = "INVALID_ARGUMENT"

	// CodeInternalError — invariant violation, panic recovery, or a
	// failure path that isn't yet mapped to a more specific Code.
	// Ships with a file-an-issue link in next_actions.
	CodeInternalError Code = "INTERNAL_ERROR"
)

// CodeMeta holds the static metadata for a Code: the agent control
// flags and the default fix. Per-occurrence detail (the wrapped
// error's message, the run_id, the log_path) is composed at emit time
// and may override the default fix when the verb has more specific
// remediation context.
type CodeMeta struct {
	Retryable          bool
	UserActionRequired bool
	DefaultFix         string
}

// catalog is the closed registry. Keep entries sorted alphabetically
// by Code so review diffs are clean and code search is predictable.
var catalog = map[Code]CodeMeta{
	CodeAborted: {
		Retryable: false, UserActionRequired: false,
		DefaultFix: "Wizard aborted by user. Re-run when ready.",
	},
	CodeActivationFailed: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "Inspect the log_path; resolve the home-manager activation conflict, then re-run `dots apply`.",
	},
	CodeBootstrapRequired: {
		Retryable: true, UserActionRequired: true,
		DefaultFix: "Workspace is missing. Clone the repo and re-run from inside it, or drop --non-interactive to consent to bootstrap.",
	},
	CodeBuildFailed: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "Inspect the log_path; the Nix build error and the offending derivation are at the bottom of the log.",
	},
	CodeConfigInvalid: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "Fix the validation error in the --config file and re-run.",
	},
	CodeConfigNotFound: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "Create the file at the given path, or omit --config to use the canonical workspace state.",
	},
	CodeDeclined: {
		Retryable: false, UserActionRequired: false,
		DefaultFix: "User declined consent. Re-run with --yes if intended.",
	},
	CodeInternalError: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "File an issue at https://github.com/sanurb/.dotfiles/issues with the run_id and log_path.",
	},
	CodeInvalidArgument: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "See `dots <verb> --help` for the accepted flags and arguments.",
	},
	CodePlanStale: {
		Retryable: true, UserActionRequired: false,
		DefaultFix: "Re-run without --plan, or regenerate the plan with `dots plan --out <file>`.",
	},
	CodePreflightFailed: {
		Retryable: true, UserActionRequired: true,
		DefaultFix: "Run `dots doctor` to see what's failing; fix the listed item, then re-run.",
	},
	CodeStateInvalid: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "Re-run `dots init` to repair the persona, or hand-edit the listed field in .dots-state.toml.",
	},
	CodeStateParseFailed: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "Edit `.dots-state.toml` to fix the syntax error, or delete it and re-run `dots init`.",
	},
	CodeUnknownCommand: {
		Retryable: false, UserActionRequired: true,
		DefaultFix: "Run `dots help` to see available verbs.",
	},
	CodeWorkspaceNotFound: {
		Retryable: true, UserActionRequired: true,
		DefaultFix: "Run `dots init` from inside a clone of the dotfiles repo, or run via `nix run github:sanurb/.dotfiles -- <verb>`.",
	},
}

// Lookup returns the static metadata for a code. The second return is
// false if the code is unknown — a programming error since every
// Code constant must be in the catalog. emit() falls back to
// CodeInternalError on miss to keep the wire shape valid.
func Lookup(c Code) (CodeMeta, bool) {
	m, ok := catalog[c]
	return m, ok
}

// AllCodes returns every registered code in lexicographic order, used
// by the completeness test that pairs Code constants with catalog
// entries.
func AllCodes() []Code {
	out := make([]Code, 0, len(catalog))
	for c := range catalog {
		out = append(out, c)
	}
	slices.Sort(out)
	return out
}
