# ADR-0003: Home Manager (and nix-darwin) as the realization layer

Realization is delegated to Home Manager for user state and reserved for nix-darwin when macOS system state is required — both are the maintained, well-documented baseline for declarative configuration on Nix. chezmoi was rejected as non-declarative; `nix-wrapper-modules` was deferred until it reaches 1.0 stability. Today only the Home Manager surface (`modules/home/`) is populated; the nix-darwin slot is a reserved seam, populated when system-level (vs user-level) state first appears.
