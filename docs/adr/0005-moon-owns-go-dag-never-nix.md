# ADR-0005: Moon owns the Go DAG, never orchestrates Nix

Moon owns the Go build/test/lint DAG under `apps/` plus repo-root tasks (formatting, repo-wide gates) declared in the top-level `moon.yml`. Moon tasks may *invoke* commands that use Nix (`nh`, `nix build`) but they never orchestrate Nix concepts — derivations, store paths, profiles. That lane discipline prevents Moon and the flake from each becoming half-owners of dependency resolution.
