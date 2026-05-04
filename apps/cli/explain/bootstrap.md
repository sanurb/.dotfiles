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
     If absent, dots offers to clone the dotfiles repo to ~/.dotfiles
     (with explicit consent). On consent it chdir's into the clone and
     resets the workspace cache so subsequent calls resolve the new root.

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
