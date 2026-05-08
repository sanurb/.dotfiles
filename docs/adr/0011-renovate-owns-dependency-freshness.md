# Renovate owns repo dependency freshness

The repo's pinned manifests (`flake.lock`, `go.mod` / `go.work.sum`,
GitHub Actions, `package.json`) are kept current by Renovate, which
opens grouped, weekly, optionally auto-mergeable PRs. The repo is the
source of truth; `git pull && dots apply` is the materialization verb
on every machine. Freshness is a property of the repo, not of any one
laptop.

## Considered Options

- **Dependabot only** (the prior state, partial) — has no Nix manager,
  so a second mechanism was already required. Rejected to avoid
  running two bots against overlapping ecosystems (github-actions,
  gomod) and to consolidate policy in one place.
- **`update-flake-lock` GH Action + Dependabot** (the prior state,
  combined) — two systems with overlapping responsibility on
  github-actions, neither covers npm or future ecosystems, and each
  has its own scheduling/grouping/labeling surface to drift.
- **`topgrade`** — solves the wrong layer. It is a machine-side
  imperative orchestrator that bumps state on whichever laptop runs
  it, including `nix`/`home_manager` steps that bypass the doctor
  gate from ADR-0003. Inverts the source-of-truth direction this
  project is built around.
- **Manual `nix flake update` etc.** — coherent but loses
  multi-machine convergence to whichever host last ran the bumps,
  and creates an invisible drift surface.

## Consequences

- Auto-merge requires real CI gates. Today: `flake-check`,
  `cli-quality` (vet/test/treefmt), `commit-lint`. The auto-merge
  policy in `.github/renovate.json5` is calibrated to those gates;
  expanding auto-merge to a new ecosystem requires adding a gate
  first, not loosening the policy.
- `npm` is **not** auto-merged because no behavioral test exists for
  `@opencode-ai/plugin`. Revisit if such a test lands.
- `flake.lock` is **not** auto-merged. nixpkgs unstable can ship
  anything; `flake-check` is the safety net, human review is the
  gate.
- `.prototools` and `config/nvim/lazy-lock.json` are explicitly
  **out of scope**. Runtime versions are deliberately manual (see
  the file header in `.prototools`); nvim plugins flow through
  `:Lazy sync` inside the editor and the lock file is human-committed.
  These exclusions are recorded in `.github/renovate.json5` so a
  future reader can see they were considered, not forgotten.
- Two superseded artifacts deleted on adoption:
  `.github/dependabot.yml` and `.github/workflows/flake-lock-update.yml`.
- New manager additions to `enabledManagers` are a deliberate scope
  decision and warrant either an ADR amendment or a successor ADR.
