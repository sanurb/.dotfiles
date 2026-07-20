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
  - `git-interceptor.ts` — force non-interactive git editors; block
    `--no-verify`
  - `worker-configuration-guard.ts` — block hand-edits to Cloudflare's
    generated `worker-configuration.d.ts` (only `wrangler types` may
    write it)
  - `whimsical.ts` — rotating whimsical spinner messages
  - `pi-cloak/` — `/cloak-status`: mask secrets in tool output using
    `agent/cloak.json` patterns
  - `save-md/` — `/save-md <name>`: save the last assistant response as
    markdown
  - `pi-skill-toggle/` — `/toggle-skills`: interactive on/off toggling of
    discovered skills
  - `web-tools/` — `webfetch` + `websearch` tools (websearch needs
    `EXA_API_KEY` in the environment)
- `agent/skills/` — pi's user-level skill root: the
  [mattpocock/skills](https://github.com/mattpocock/skills) set (vendored;
  `herdr` excluded because it is already installed in `~/.agents/skills`)
  plus the Cloudflare docs bundle.
- `agent/themes/` — `catppuccin-macchiato` (select via
  `"theme": "catppuccin-macchiato"` in `~/.pi/agent/settings.json`).
- `agent/mcp.json` — MCP servers (proxy tool mode, lazy startup):
  codebase-memory-mcp, context7, grep_app, opensrc.
- `agent/cloak.json` — secret-masking patterns (env files, opencode
  apiKeys, auth.json tokens, Cloudflare Access vars).

## Workflow

```sh
cd config/pi
npm install        # once, and after adding extension deps
npm run check      # typecheck + tests across extension workspaces
```

Edits land on pi's next session start; use `/reload` inside a running
session.
