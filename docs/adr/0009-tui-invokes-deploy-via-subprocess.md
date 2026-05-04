# ADR-0009: The TUI invokes `dots deploy` via subprocess

After the install/sync wizard captures `selection.toml` and finishes brownfield scan + snapshot, the user is asked "Realize now? [Y/n]". On Yes, the wizard exits the bubbletea program cleanly and `main.go` invokes `dots deploy` as a subprocess (resolving the binary via `os.Executable()` with `exec.LookPath("dots")` as fallback, inheriting stdio).

This is **not** an I1 violation. I1 says the TUI's import graph is Nix-free, and that holds: `os/exec` invoking a sibling subcommand encodes only the existence of the subcommand, not any Nix concept. AGENTS.md §8 was tightened in this PR to make the import-vs-subprocess distinction explicit.

The subprocess approach is preferred over an in-process call to `runDeploy()` for three reasons. First, `dots deploy`'s dispatcher gates (workspace check today, auto-bootstrap consent in PR #3) re-fire on every invocation — the post-wizard realize follows the same path as a manually-typed `dots deploy`, and PR #3's bootstrap behavior automatically benefits the install flow without re-wiring. Second, `nh home switch` (and `nom`'s renderer) draws against a real terminal; running it inside a still-active alt-screen TUI corrupts both surfaces, and exiting bubbletea before re-entering with `nh` is structurally cleaner than mid-program Suspend/Resume gymnastics. Third, the cleanly-distinct exit codes from each phase (130 from a wizard SIGINT vs. an `nh` failure exit code) survive without bookkeeping.

Trade-off: a process boundary loses Go-typed error context. Mitigated by inheriting stderr — `nh`'s own diagnostics reach the user verbatim — and by `main.go` propagating the subprocess exit code instead of inventing one.
