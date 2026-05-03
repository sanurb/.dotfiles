# `libs/` — shared Go libraries

Empty by design. Reserved for code that >=2 apps import.

## When to populate

A new `libs/<name>/` is justified when **>=2 apps import the same logic**,
or **1 app imports it AND a second app is imminent** (concrete plan, not
"someday"). Until then, code stays in its single consumer
(`apps/cli/internal/...`).

This is the same >=2-consumer rule that governs `.moon/tasks/tag-*.yml`
inheritance files.

## Onboarding

```sh
mkdir libs/<name>
cat > libs/<name>/moon.yml <<EOF
\$schema: "https://moonrepo.dev/schemas/project.json"
type: library
language: go
tags: [go]
project:
  name: <name>
  description: "..."
EOF
```

`tidy` / `lint` / `test` / `build` are inherited from `tag-go.yml`. No
`.moon/workspace.yml` edit — the `libs/*` glob picks it up.

`libs/.keep/` is a hidden placeholder so the directory tracks in git
without tripping Moon's "files in project glob" warning. Delete it when
the first real library lands.
