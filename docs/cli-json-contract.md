# dots `--json` contract

This is the stable JSON contract the `dots` CLI emits on stdout when
`--json` is set. Agents (Claude Code, opencode, scripts) parse it
directly. Humans pipe through `jq`. There is **no second format**
under `--json`: the envelope is the only thing that hits stdout.

## Stability

The contract is **forever once shipped**. New fields are added freely
(omitempty); existing fields are never removed or renamed. There is
no schema_version field — the shape itself is the contract, and
additive evolution is the migration plan.

If a backwards-incompatible change is ever required, it ships under a
new top-level discriminator (e.g., a `--json-v2` flag) with a real
deprecation window. Until that day, every consumer that worked on the
day this contract shipped keeps working.

## Envelopes

Every command's terminal stdout line is one of two envelopes.

### Success — snapshot command

Snapshot commands return synchronously without producing a per-run
log. Examples: `dots status`, `dots profile show`, `dots plan`,
`dots why`, `dots capture`, `dots apply --dry-run`.

```json
{
  "ok": true,
  "command": "dots status",
  "result": {
    "workspace": "/Users/sanurb/src/dotfiles",
    "profile": "fish · ghostty · none",
    "drift": "converged"
  },
  "next_actions": [
    {
      "command": "dots apply",
      "description": "Realize the profile."
    },
    {
      "command": "dots doctor",
      "description": "Audit the realized environment against the declared persona."
    }
  ]
}
```

### Success — long-running command

Long-running commands stream NDJSON intermediate events and finish
with a terminal envelope that adds `run_id` (ULID) and `log_path`.
Examples: `dots apply`, `dots sync`, `dots update`, `dots rollback`.

```json
{
  "ok": true,
  "command": "dots apply",
  "run_id": "01HV2K3X9Y4Z5A6B7C8D9E0F1G",
  "log_path": "/Users/sanurb/.dots_logs/01HV2K3X9Y4Z5A6B7C8D9E0F1G/apply.log",
  "result": {
    "steps_executed": 4,
    "profile_hash": "9e57fb1fe815"
  },
  "next_actions": [
    { "command": "dots status", "description": "Confirm convergence." }
  ]
}
```

### Error

The error envelope adds three control-flow fields the agent branches
on, plus a remediation string an agent can execute or surface.

| Field | Purpose |
|---|---|
| `error.code` | SCREAMING_SNAKE id from the closed catalog (below). Fine-grained pattern match. |
| `error.message` | Per-occurrence human-readable detail. |
| `retryable` | True if retrying the same command might succeed (e.g., transient or user-fixable). |
| `user_action_required` | True if the user must intervene before any retry. |
| `fix` | Plain-language remediation. Catalog default unless the verb supplies a more specific fix. |
| `next_actions` | Contextual follow-ups, same shape as success. |
| `run_id` / `log_path` | Present only when the failing command is long-running. |

```json
{
  "ok": false,
  "command": "dots init --config /missing.toml",
  "error": {
    "message": "config file not found: /missing.toml",
    "code": "CONFIG_NOT_FOUND"
  },
  "retryable": false,
  "user_action_required": true,
  "fix": "Create the file at the given path, or omit --config to use the canonical workspace state.",
  "next_actions": [
    {
      "command": "dots init [--config <path>]",
      "description": "Re-run init with a valid config or none.",
      "params": {
        "path": { "description": "Path to a .dots-state.toml seed file." }
      }
    }
  ]
}
```

#### Agent decision matrix

The two boolean flags map to the four agent behaviors the CLI ever
needs to express:

| `retryable` | `user_action_required` | Agent should |
|:---:|:---:|---|
| true | false | Retry the command as-is (likely transient). |
| true | true | Surface the `fix` to the user; retry after they act. |
| false | true | Escalate to user; do not retry. |
| false | false | Terminal user-initiated decision (e.g., declined consent). Do not retry, do not escalate. |

## Streaming (long-running commands)

When `--json` is set on a long-running verb, intermediate progress is
emitted as NDJSON: one JSON object per line, terminated by `\n`. The
**last line is always the terminal envelope** (success or error,
shapes above). `jq -s 'last'` extracts it for consumers that don't
care about progress.

The terminal line has no `type` field — its absence (and the presence
of `ok`) is the discriminator. Intermediate lines always carry
`type`.

### Event vocabulary

```
start  — stream began; echoes the command
step   — phase started/completed/failed; carries name and duration
```

The vocabulary is intentionally minimal. There is no `progress`
event: phase boundaries via `step` are what the dots pipeline
actually has signal for; emitting fake progress would couple the
contract to nh's stdout format. There is no `log` event: per-line
diagnostic output goes to the per-run log file at `log_path` to
preserve agent context windows.

### Example transcript

```
{"type":"start","ts":"2026-05-07T02:30:00Z","command":"dots apply"}
{"type":"step","ts":"2026-05-07T02:30:00.012Z","name":"snapshot-conflicts","status":"started"}
{"type":"step","ts":"2026-05-07T02:30:00.135Z","name":"snapshot-conflicts","status":"completed","duration_ms":123}
{"type":"step","ts":"2026-05-07T02:30:00.140Z","name":"apply-profile","status":"started"}
{"type":"step","ts":"2026-05-07T02:30:42.892Z","name":"apply-profile","status":"completed","duration_ms":42752}
{"ok":true,"command":"dots apply","run_id":"01HV2K3X9Y4Z5A6B7C8D9E0F1G","log_path":"/Users/sanurb/.dots_logs/01HV2K3X9Y4Z5A6B7C8D9E0F1G/apply.log","result":{"steps_executed":2}}
```

A `step` event with `status: "failed"` does not include error detail
inline — the terminal error envelope carries the prose, the catalog
code, and the remediation. The step event only marks the structural
boundary.

## `next_actions` template syntax

Each action's `command` is either a literal string (run as-is, no
`params`) or a POSIX/docopt template with placeholders that the agent
fills from `params`:

| Syntax | Meaning |
|---|---|
| `<name>` | Required positional or named value. |
| `[--flag <value>]` | Optional flag with a value. |
| `[--flag]` | Optional boolean flag. |

The `params` map describes each placeholder:

| Field | Purpose |
|---|---|
| `description` | Human-readable explanation. |
| `value` | Pre-filled from current context (the action emitter knows the concrete value). |
| `default` | The value when the flag is omitted. |
| `enum` | Closed set of accepted values. |
| `required` | True for positionals; absent or false for optional flags. |

When `params` is absent the command is a literal — no substitution.

## Error code catalog

The closed set of `error.code` values, with their default agent
control-flow flags.

| Code | retryable | user_action | When |
|---|:---:|:---:|---|
| `ABORTED` | false | false | Wizard aborted (Ctrl-C, Cancel). |
| `ACTIVATION_FAILED` | false | true | Build OK, home-manager activation step failed. |
| `BOOTSTRAP_REQUIRED` | true | true | Workspace missing; bootstrap requires interactive consent. |
| `BUILD_FAILED` | false | true | Nix evaluation or derivation build failed. |
| `CONFIG_INVALID` | false | true | `--config` parsed but failed validation. |
| `CONFIG_NOT_FOUND` | false | true | `--config PATH` did not resolve. |
| `DECLINED` | false | false | User declined a consent prompt. |
| `INTERNAL_ERROR` | false | true | Invariant violation; ships with a file-an-issue link. |
| `INVALID_ARGUMENT` | false | true | Flag value or positional outside the verb's accepted shape. |
| `PLAN_STALE` | true | false | `--plan FILE` hash diverged from a freshly-computed plan. |
| `PREFLIGHT_FAILED` | true | true | `dots doctor` flagged a SevFail before activation. |
| `STATE_INVALID` | false | true | `.dots-state.toml` parsed but a pillar value is outside its closed set. |
| `STATE_PARSE_FAILED` | false | true | `.dots-state.toml` is syntactically malformed. |
| `UNKNOWN_COMMAND` | false | true | Unknown verb or alias. |
| `WORKSPACE_NOT_FOUND` | true | true | Verb requires a cloned dotfiles workspace; none was found. |

The catalog is the single source of truth. It lives at
`apps/cli/internal/envelope/catalog.go` and is exercised by
`TestCatalogIsComplete` — a Code constant without metadata fails the
build.

## What is intentionally not in the contract

These were considered and dropped, each with a real reason. None of
them are forbidden forever — when a real reader asks for one, it is
added (additive evolution).

| Field | Cut reason |
|---|---|
| `schema_version` | Ceremony. The shape is the contract; additive evolution is the migration plan. |
| `type` (docs URL) | Without versioning, the URL is just lookup convenience. The catalog is the index. |
| `error_category` | Redundant with `retryable` + `user_action_required` for agent control flow. dots is one local system, not heterogeneous backends. |
| `exit_code` | The shell knows the exit code; programmatic callers reading the JSON already have the wait status. |
| `retry_after` | Meaningful only for network rate-limits. dots is local; no scenario produces a meaningful retry window. |
| `progress` event | Would require parsing nh's stdout format; coupling the contract to a downstream tool's output is fragile. |
| `log` event | Inlining nh's stderr line-by-line would blow the agent's context. Logs go to `log_path`. |
