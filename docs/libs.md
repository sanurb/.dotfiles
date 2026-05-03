# `libs/` — shared Go libraries (currently empty)

The `libs/` directory at the repo root is reserved for shared Go
libraries that more than one app imports. It exists empty by design —
this document explains why, and the rules for populating it.

## Why the directory exists empty

`apps/lsp` (a Language Server for `selection.toml`) is the next planned
binary. Once it lands, it and `apps/cli` will both need a declarative
schema; that schema is the first thing that will move into
`libs/schema/`. The directory is the structural commitment to that
extraction. **No code moves into `libs/` in the PR that creates this
directory** — that is a separate, scoped extraction.

Until then, code lives inside its single consumer
(`apps/cli/internal/...`).

## When to add a `libs/<name>/`

A new library is justified when:

- **≥2 apps import the same logic**, OR
- **1 app imports it AND a second app is imminent** (concrete plan, not
  "someday").

This is the same ≥2-consumer rule that governs `.moon/tasks/tag-*.yml`
inheritance files. Premature extraction is a Maintainability anti-pattern
(priority #3 in `AGENTS.md`), not a Maintainability win — it adds an
abstraction layer that has not yet earned its weight, then accumulates
shape based on speculation about the second consumer rather than its
real requirements.

## How to add a `libs/<name>/`

1. `mkdir libs/<name>`
2. Drop a `moon.yml`:

   ```yaml
   $schema: "https://moonrepo.dev/schemas/project.json"
   type: library
   language: go
   tags:
     - go

   project:
     name: <name>
     description: "..."
   ```

3. Tag-based inheritance from `.moon/tasks/tag-go.yml` provides
   `tidy` / `lint` / `test` / `build`. No `.moon/workspace.yml` edit is
   required — glob discovery (`libs/*`) picks it up automatically.

That's the entire onboarding path. If it ever requires more steps, the
Moon configuration has drifted and needs to be re-aligned.

## Why `libs/` itself ships with a placeholder directory

Moon's project glob (`libs/*`) treats every entry under `libs/` as a
potential project root and warns when it encounters a regular file. To
keep the directory tracked in git without tripping that warning, `libs/`
contains a single hidden directory (`libs/.keep/`) holding a `.gitkeep`
sentinel. When the first real library lands, `.keep/` can be deleted.
