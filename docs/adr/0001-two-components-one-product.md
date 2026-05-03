# ADR-0001: Two components, one product

The TUI ships standalone (Homebrew, curl-install, `nix run`) so users can try the wizard without committing to Nix; the realization layer requires Nix because reproducibility is non-negotiable. The asymmetry is the reason no single-command installer can honestly cover the full product — the entry-point × depth split in AGENTS.md §7 is a property of the architecture, not a UX accident to paper over.
