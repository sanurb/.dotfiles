# `dots apply` self-bootstraps Nix and the workspace clone behind explicit consent

When `dots apply` (or its `init` alias) runs on a host without Nix or
without the workspace clone, it offers — one prereq at a time, exit
code `3` on decline — to install Nix (Determinate Systems installer)
and clone the workspace. The exact command is printed before each
`[y/N]`. The alternative — exit `2` with a copy-paste recipe — broke
the single-invocation install promise and was a dead end for the
wizard's post-consent subprocess hand-off.

## Consequences

Honesty rule: every install is a separate, named consent prompt with
the literal command shown; `dots` never silently fetches. Bypass uses
`--yes`, which still prints what it auto-approved.
