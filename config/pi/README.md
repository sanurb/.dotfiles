# pi agent config

Live-edit config for [pi](https://github.com/badlogic/pi-mono), symlinked
into `~/.pi/agent/` per-surface by `modules/home/pi.nix` (extensions,
skills, themes, mcp.json, cloak.json). Runtime state (auth.json,
sessions/, settings.json, trust.json) stays machine-local in `~/.pi/agent/`
and is never tracked here.

Adapted from [dmmulroy/.dotfiles](https://github.com/dmmulroy/.dotfiles)
(`home/.pi/`).

## Layout

- `agent/extensions/` — auto-discovered from `*.ts` and `*/index.ts`
  (packaged extensions declare entry points via `pi.extensions` in their
  package.json). This directory is an npm workspace rooted at
  `config/pi/package.json`.
  - `answer.ts` — `/answer`: extract unanswered questions from the last
    assistant message into an interactive editor
  - `continue-after-compaction.ts` — auto-resume the task after a
    successful compaction
  - `herdr-codemode/` — deferred `herdr_*` SDK tools and `herdr_search` for
    codemode; enabled inside Herdr, or forced with `PI_HERDR_CODEMODE=1`
    (`0` disables it)
  - `pi-worktrees/` — `/worktrees`: list, create, fetch, and safely remove
    worktrees in the canonical `.bare` layout; uses the repo-owned worktrees skill
  - `cfpaste/` — `/cf-paste` and `/cf-paste-last`; disabled until
    `PI_CFPASTE_ORIGIN` selects an authorized HTTPS Paste service
  - `private-gateway/` — identity-protected inference providers; registers
    nothing without machine-local `~/.pi/agent/private-gateway.json`
  - `cloudflare-deployment-allowlist.ts` — fail-closed deployment guard;
    machine-local policy lives at `~/.pi/agent/cloudflare-deployment-allowlist.json`
    (or Pi's overridden agent directory), never in this repo
  - `git-interceptor.ts` — force non-interactive git editors; block
    `--no-verify`
  - `worker-configuration-guard.ts` — block hand-edits to Cloudflare's
    generated `worker-configuration.d.ts` (only `wrangler types` may
    write it)
  - `whimsical.ts` — rotating whimsical spinner messages
  - `pi-cloak/` — `/cloak-status`: mask secrets in tool output using
    `agent/cloak.json` patterns
  - `save-md/` — `/save-md <name>`: save the last assistant response as
    markdown and queue the saved path in the next turn's context
  - `pi-skill-toggle/` — `/toggle-skills`: interactive on/off toggling of
    discovered skills
  - `web-tools/` — `webfetch` + `websearch` tools (websearch needs
    `EXA_API_KEY` in the environment)
- `agent/skills/` — pi's user-level skill root: the
  [mattpocock/skills](https://github.com/mattpocock/skills) set, the Cloudflare
  docs bundle, and upstream Herdr/worktree/decision-model/review skills.
  Herdr is vendored here rather than linked to an installer-owned skill root;
  a fresh clone no longer contains a dangling upstream-layout symlink.
  - `coding-standards/` retains upstream's final split bundle: a short
    `SKILL.md` indexing `references/`, including the Alchemy and lint-policy
    guidance. Keep the references split so the agent loads only what it needs.
    Upstream later removed this skill; its retained revision and the other
    historical skill sources are recorded in
    [the sync notes](../../docs/upstream-sync.md#skill-source-coverage).
    Its `disable-model-invocation: true` flag follows that final revision.
  - `recipe-diagrams/` ships `scripts/render_recipe_diagram.py`
    (stdlib-only, needs `python3`) and an optional PNG renderer that requires
    ImageMagick.
  - `cua-driver/` is pinned to Cua Driver v0.24.0 and documents the
    snapshot-action-verify loop used by the persistent `computer` MCP server.
  - Deliberately NOT vendored from upstream: `workday-training` (marks Workday
    compliance courses passed
    via the SCORM API without taking them — falsifies auditable training
    records).
- `agent/themes/` — `catppuccin-macchiato` (select via
  `"theme": "catppuccin-macchiato"` in `~/.pi/agent/settings.json`).
- `agent/mcp.json` — native Pi 1.4 MCP servers, exposed through codemode:
  computer (Cua Driver) and grep_app.
  Local servers require their executable on PATH.
  Pi connects enabled servers in the background; MCP timeouts are in seconds.
- `agent/cloak.json` — secret-masking patterns (env files, opencode
  apiKeys, auth.json tokens, Cloudflare Access vars).

## Workflow

```sh
cd config/pi
npm install        # development dependencies for extension workspaces
npm run check      # typecheck + tests across extension workspaces
```

`dots apply` checks and installs runtime dependencies for the root and every
extension workspace, including Herdr's pinned Effect runtime. It also upgrades
older Bun-installed Pi binaries to the compatible 1.4 release line. Pi discovers
local extension source but, unlike `pi install`, does not install its dependencies.

`npm run check` typechecks the flat extensions and each packaged workspace, then
runs workspace tests and the standalone guard/compaction tests. Herdr tests use
an isolated fixture socket, not a live terminal session. Gateway tests use local
fixtures, not configured inference services.

See [upstream sync notes](../../docs/upstream-sync.md) for the source revision,
local adaptations, deliberately excluded machine-specific files, and remaining
Nix verification.

Edits land on pi's next session start; use `/reload` inside a running
session.
