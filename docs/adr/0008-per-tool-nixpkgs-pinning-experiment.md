---
status: proposed
---

# ADR-0008: Per-tool `nixpkgs` pinning as a six-month experiment

Some tools are pinned to per-tool `nixpkgs` revisions rather than the workspace-wide pin, so an upstream regression in one tool does not drag the whole workspace back to an old `nixpkgs`. This is explicitly time-boxed: the maintenance log lives in `docs/maintenance.md`, and if fewer than three stuck-update incidents are recorded over six months the experiment collapses back to a single workspace pin (the per-tool overhead was not worth it).
