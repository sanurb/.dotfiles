# Cua Driver — dots integration

## Transport

Use the persistent `computer` MCP server configured in
`config/pi/agent/mcp.json`; Home Manager exposes that file at
`~/.pi/agent/mcp.json`. Its tools have the `computer_` prefix. Prefer MCP for
multi-call GUI workflows; use the CLI for installation, diagnostics, and
isolated inspection. Do not mix one-shot CLI calls into an MCP action sequence.

Discover with `mcp({ search: "...", server: "computer" })`, describe an
unfamiliar tool once, then reuse its schema. Pass `mcp.args` as serialized JSON:

```text
mcp({ tool: "computer_get_window_state", args: "{\"pid\":844,\"window_id\":10725}" })
```

The installed tool schemas are authoritative when upstream examples disagree.
Use exact window IDs and fresh snapshot-bound element tokens. Verify after each
action; never retry text insertion before inspecting whether it landed. On a
connection error, reconnect once and rediscover. On a permission error, stop
and ask the human to grant the required OS permission.

## Desktop and data safety

Prefer a purpose-built API, CLI, or filesystem operation for non-GUI outcomes.
Use Cua for authenticated UI, desktop-only work, and visual verification.

Keep background delivery and the visible agent cursor. Ask before foreground or
desktop takeover unless the user already authorized it for this workflow. This
also applies to temporarily activating native menu operations. Do not substitute
AppleScript or global-pointer tools after a Cua refusal.

Do not enable recordings or history, or widen runtime permissions,
automatically. The Home Manager installer selects standard permission mode and
disables telemetry during installation.

## Upstream provenance and updates

The skill is vendored from `trycua/cua`, tag `cua-driver-rs-v0.24.0`, path
`libs/cua-driver/rust/Skills/cua-driver`. `modules/home/pi.nix` pins the matching
driver version and installs it through Cua's official installer so macOS keeps
the signed `/Applications/CuaDriver.app` identity required for Accessibility
and Screen Recording grants.

Canonical skill files live under `config/pi/agent/skills/cua-driver/`. When
updating, keep this file and the local entry-point note in `SKILL.md`, and review
the guidance against the installed binary.
