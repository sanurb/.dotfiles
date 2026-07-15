# dots explain: bootstrap

`dots install` is the single supported entry point for a fresh
machine. It auto-bootstraps Nix and the workspace clone behind explicit
per-prereq consent and runs the wizard against the resulting workspace
— there is no "standalone" branch that produces a TOML artifact and
three manual steps.

What "bootstrap" means here, in order:

1. Nix on PATH?
   If absent, dots offers (with explicit consent) to install Nix via
   the Determinate Systems installer. On consent, Nix is installed,
   and dots exits cleanly: the new `nix` binary is not on the current
   process's PATH, so the user is told to open a new shell and re-run
   `dots install`.

2. Workspace cloned?
   dots resolves the workspace from anywhere: the cwd walk finds a
   checkout you are inside, and failing that it falls back to the
   canonical location ($DOTS_WORKSPACE, else ~/.dotfiles). An existing
   checkout there is adopted, not re-cloned — running `dots` from your
   home directory never tries to clone over ~/.dotfiles.
   Only when no workspace exists does dots offer to clone it (with
   explicit consent), then chdir into the clone and reset the workspace
   cache so subsequent calls resolve the new root. A non-empty target
   that is not a dots workspace is reported as an actionable error
   rather than a raw `git clone` failure.

   nh (the activation driver) does not need to be pre-installed. When
   it isn't already on PATH or in <workspace>/.devenv/profile/bin, apply
   provisions it from the flake's pinned dev shell via `nix develop -c
   nh …` — so a machine with only Nix converges without `direnv allow`
   first.

3. Wizard.
   With Nix and the workspace present, the wizard runs:
   capabilities -> conflicts -> deploy
   producing a `.dots-state.toml` at the workspace root and, on
   consent, invoking `dots deploy` as a subprocess.

`dots init` (when implemented) is the non-interactive sibling: same
prereq logic, no wizard, intended for unattended provisioning.

Honesty rules:

- Every prereq install is a separate, named consent. dots never
  silently fetches.
- Failure to consent exits 2 (misuse — the operator chose to abort
  a setup they invoked themselves).
- dots never edits the user's shell rc files outside the Home Manager
  activation path.
