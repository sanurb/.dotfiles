# Reference dotfiles sync

## Source and layout

Reference: [dmmulroy/.dotfiles](https://github.com/dmmulroy/.dotfiles), pinned for
this import to [`24e8ae72373830197349c55d6bd901e7d5d61436`](https://github.com/dmmulroy/.dotfiles/tree/24e8ae72373830197349c55d6bd901e7d5d61436).
The Herdr codemode integration originates in
[`4b4d38a4130650bd92783f173691e9e58901e708`](https://github.com/dmmulroy/.dotfiles/commit/4b4d38a4130650bd92783f173691e9e58901e708).

This is an adaptation, not a replacement of the Nix-native platform with
upstream's macOS/Homebrew/Stow installer. Upstream `home/.pi/` maps to
`config/pi/`; `home/.agents/skills/` maps to `config/pi/agent/skills/`;
`home/.config/` maps to `config/`. Neovim's upstream `dmmulroy` Lua namespace
remains `dots` here. Unchanged existing configurations are retained.

## Skill source coverage

The skill audit checks upstream main at
[`8cdeee1b03b0d8f038a89080c5a1fd660875e349`](https://github.com/dmmulroy/.dotfiles/tree/8cdeee1b03b0d8f038a89080c5a1fd660875e349),
including history for retained skills absent from that tree. Comparing only
directories present at HEAD misses changes made before a skill was removed.
The broader audit refreshed 15 retained bundles; it did not remove locally
retained skills or install any services.

Twelve included skills are still tracked at that revision: bro, cua-driver,
decision-models, diagnosing-bugs, handoff, herdr, plannotator-tui,
recipe-diagrams, show-me, worktrees, write-discoverable-code, and
writing-for-agents. Cua Driver retains its documented local integration.

The following skills remain available locally even though upstream removed
them. Each source snapshot below is the parent of that skill's latest removal
commit, so it contains the final tracked bundle, not an older installer copy.
These are **historical pins**, not claims that the skills remain maintained
in upstream main.

| Source snapshot | Retained skills |
| --- | --- |
| [`4a783032e8f1254759ed9e97b65f6108478f0cb6`](https://github.com/dmmulroy/.dotfiles/tree/4a783032e8f1254759ed9e97b65f6108478f0cb6/home/.agents/skills) | cloudflare-composition-root, coding-standards, prototype, research |
| [`7c086fe40c01bf494aea7184da2eb7aaf7b7dff4`](https://github.com/dmmulroy/.dotfiles/tree/7c086fe40c01bf494aea7184da2eb7aaf7b7dff4/home/.agents/skills) | code-review, domain-modeling, grilling, grill-me, grill-with-docs, tdd |
| [`be341f044a0c56b10ab25d89e8bf0762909927ef`](https://github.com/dmmulroy/.dotfiles/tree/be341f044a0c56b10ab25d89e8bf0762909927ef/home/.agents/skills) | ask-matt, setup-matt-pocock-skills, to-spec, triage, wayfinder |
| [`c2322c6534f586b146ae0e8d9296019396aa32c0`](https://github.com/dmmulroy/.dotfiles/tree/c2322c6534f586b146ae0e8d9296019396aa32c0/home/.agents/skills) | bootstrap-prelude, codebase-design, improve-codebase-architecture, prelude, teach, tech-spec, to-tickets |
| [`b0e4535dc9a8432be71e577359a0b8975bf157b6`](https://github.com/dmmulroy/.dotfiles/tree/b0e4535dc9a8432be71e577359a0b8975bf157b6/home/.agents/skills) | implement |
| [`fd4b03e71e80f8dd54e36014b87943cd75c67677`](https://github.com/dmmulroy/.dotfiles/tree/fd4b03e71e80f8dd54e36014b87943cd75c67677/home/.agents/skills) | resolving-merge-conflicts |
| [`fdd5aedabb05e4d752d80472be0bd4230fb56dfd`](https://github.com/dmmulroy/.dotfiles/tree/fdd5aedabb05e4d752d80472be0bd4230fb56dfd/home/.agents/skills) | writing-great-skills |

The retained coding standard includes all 23 reference files, including
`effect-alchemy.md` and `lint-and-policy.md`. Its final
`disable-model-invocation: true` setting is preserved, as are those settings
for cloudflare-composition-root, prototype, and research. Skills do not
override governing repository or global instructions, including permission
boundaries and one-question-at-a-time interaction rules.

Existing local adaptations remain explicit: research's exactly-one-agent
wording and domain-modeling's corrected heading separation. Upstream's final
domain-modeling file concatenates `## During the session` onto the preceding
paragraph; that Markdown defect is not reintroduced. The other retained
bundles match their source snapshots verbatim. Code-review's recursion
prevention prompt and research's recursion guard come from upstream.

The 11 Cloudflare-bundle skills with no history at upstream's shared skill
path are unchanged and outside this parity claim: agents-sdk, cloudflare,
cloudflare-email-service, cloudflare-one, cloudflare-one-migrations,
durable-objects, sandbox-sdk, turnstile-spin, web-perf,
workers-best-practices, and wrangler. The older monolithic coding standard
in `dmmulroy/skills` is not substituted for the later split `.dotfiles`
bundle.

## Imported behavior

- **Pi:** Herdr codemode, worktree management, optional Cloudflare Paste,
  optional private gateways, deployment allowlisting, and standalone guard
  tests. Existing compaction continuation now steers the retried turn;
  `/save-md` also queues the saved path in next-turn context; skill discovery
  deduplicates symlinked roots/files. Worker-configuration detection receives
  upstream's narrower, high-confidence shell-mutation checks.
- **Herdr SDK:** upstream's built SDK, declarations, schema, and wire-method map
  are vendored unchanged. Its own `VENDORED.md` records SDK revision/version.
  The SDK\u0027s MIT license is retained and copied by the refresh script;
  that script requires an explicit checkout rather than a personal path.
  Formatter exclusions protect the generated artifacts. Tests use an isolated
  socket and cover the exposed operation surface, not a live Herdr session.
- **Skills:** decision-models, Herdr, plannotator-tui, show-me, worktrees, and
  writing-for-agents; refreshed diagnosing-bugs, handoff, and recipe-diagrams,
  including the optional ImageMagick-backed PNG renderer. The broader history
  audit also refreshes retained bundles described above, including coding
  standards, prototype, research, and the planning/review skills. Cua Driver
  remains v0.24.0 with the existing local transport/safety guidance preserved.
- **Fish:** refreshed Bun completions, lazy Vite+ completions, Vite+ environment
  switching, worktree helpers (`wt`, `wtcd`, `wtd`, `wtl`, `wtp`, `wtr`), and
  pnpm's Vite+ `dlx` shortcut. Proto shims retain PATH priority; pnpm falls back
  to itself when `vpx` is absent. Existing aliases and theme remain intact.
- **Herdr/review:** notification navigation, annotation plugin bindings,
  optional plugin sources, Plannotator preferences and foreground review
  commands, and OpenCode's Herdr TUI session bridge. Runtime sockets, plugins,
  generated integration shims, and review history remain machine-local.
- **Editor/Git/macOS:** secret masking for Fish secrets, IdeaVim preferences,
  commit-graph writing, branch/column sorting, rebase ref updates, local-agent
  ignore patterns, and optional macOS Tinycast preferences. Tinycast is not
  installed or trusted automatically.

## Local integration and compatibility

Pi dependencies require the 1.4 release line. Home Manager checks dependencies
from the root and every extension manifest instead of installing only
`web-tools`; its Bun hook upgrades older Pi binaries. The dependency probe uses
Node's import condition so ESM-only Pi packages resolve without being executed
or triggering a needless install on every activation. The root check script
includes both standalone extensions and package workspaces, each using its own
compiler settings. npm regenerates the lockfile; it is not edited manually.

The imported private gateway needs Pi 1.4's classifier transport compatibility
field. Its now-obsolete Clef Omni overlay is removed so Pi's catalog supplies
native metadata/capabilities. Tests cover image serialization as well as Access
headers and origin enforcement. The local `/answer` extension derives its auth
registry type from Pi rather than duplicating the older headers contract.

Imported implementations retain upstream module/API names and schema-boundary
casts rather than introducing a parallel local SDK abstraction. Strictness is
scoped to each workspace. The flat-extension config and Herdr workspace retain
their existing optional-property semantics: a stricter diagnostic probe found
five flat-extension errors (legacy `/answer` optional fields/options and a guard
test's optional environment) and three Herdr errors (two mutually exclusive SDK
input casts and optional `spread`). Migrating those contracts is separate from
this import; the new paste/worktree/provider configurations use exact optional
properties. Herdr also enables override and switch-fallthrough checks.

The worktree extension resolves its creation script from the vendored skill,
not an assumed `~/.agents/skills` installation. The previously tracked Herdr
skill link pointed into the absent upstream directory layout; it is replaced
with the vendored skill.

### Opt-in services and review tools

- Herdr tools register inside Herdr; `PI_HERDR_CODEMODE=1` forces registration
  and `0` disables it. Runtime acquisition is lazy and session-scoped.
- Paste commands register only when `PI_CFPASTE_ORIGIN` names an HTTPS origin
  without credentials/path/query/fragment. No corporate endpoint is selected
  automatically. Invoking a command uploads its Markdown; review it first.
- Private gateway profiles live in Pi's machine-local agent directory. Missing
  profiles register no provider. Credentials stay in auth storage, not the repo.
- The deployment guard reads `cloudflare-deployment-allowlist.json` from Pi's
  agent directory (`~/.pi/agent` by default), not beside symlinked source. Missing
  policy denies deployments; dry runs and unrelated commands remain available.
  Configure policy manually outside the agent session. Do not commit live
  policy or gateway settings.
- Install optional Herdr plugins explicitly from `config/herdr/plugins.txt`
  with `herdr plugin install <source>`. The plugin list is not an auto-installer.
  Review commands require separately available Plannotator/Plannotator TUI.

## Deliberately not mirrored

- Upstream's `dot` installer, Brewfile, generated `dot` completions, and root
  instructions: they belong to a different platform/installation model.
  Application/package parity with its macOS workstation is not claimed.
- Personal Git identity, signing keys, work-specific configuration, credential
  helpers, certificate paths, skill installation locks, agent-wide personal
  instructions, and session todo files.
- Generated Herdr agent-state shims: the existing ignore rules intentionally
  leave them to `herdr integration`.
- Upstream's minimal OpenCode JSONC files: their additions are merged into the
  existing JSON configuration instead of shadowing local permissions/settings.
- Ghostty/Fish visual defaults and machine-owned shell hooks: retain the
  Gentleman/Iosevka theme, cross-platform Homebrew guards, Home Manager's
  direnv/starship/zoxide initialization, and Proto PATH ownership. Do not
  duplicate initialization or replace these with upstream macOS-only paths.
- Upstream's Neovim Mason auto-installer and tmux navigation fallback: retain
  Nix-provided tools and the local `NvimTmuxNavigate*` compatibility fix.
- Upstream's `agent-repos` bootstrap change: retain the local `git commit --only`
  fix that avoids committing pre-staged user files.
- The macOS `coffee` script: its shared `/tmp/coffee.pid` and unscoped
  `pkill -f caffeinate` fallback can stop unrelated keep-awake processes.
- `workday-training`: the existing exclusion remains because it falsifies
  completion of auditable compliance training.

## Verification boundary

The import is repository-only. No Home Manager activation, Herdr plugin
installation, remote paste, inference request, deployment, or system package
installation is part of it. npm dependencies may be installed locally under
`config/pi` to run extension checks, with lifecycle scripts disabled.

On a Nix-enabled machine, run the project gates before activation:

```sh
nix develop -c nix flake check
nix develop -c go build ./apps/cli/...
nix develop -c go test ./apps/cli/...
nix develop -c sh -c 'cd config/pi && npm ci --ignore-scripts && npm run check'
nix fmt
```

Skill-audit checks:

- 34 of the 37 included bundles with upstream history match their selected
  source snapshots byte-for-byte. The three exceptions are Cua Driver's
  local integration, research's exactly-one-agent wording, and the corrected
  domain-modeling Markdown described above. Executable permissions match.
- All 15 refreshed entrypoints pass basic frontmatter delimiter/name/description
  checks. All 98 relative Markdown file links in those bundles resolve;
  this file-existence check does not validate every heading anchor.
- `git diff --check` passes. The three Nix gates and `nix fmt` each fail before
  execution with `nix: command not found` (exit 127). No replacement host-side
  build, test, formatter, or skill runtime invocation was used.

Recorded host-side extension checks from the earlier import
(Node 26.12.0, npm 11.20.0):

- `cd config/pi && npm ci --ignore-scripts --no-audit --no-fund && npm run check`:
  workspace/flat-extension typechecks and 165 tests passed, none skipped.
- Isolated extension discovery: 14 extensions loaded without errors, including
  106 registered Herdr tools. No session or live Herdr connection was started.
- The embedded activation dependency probe passed against the installed tree,
  rejected a missing extension-only dependency, and resolved an ESM-only fixture
  without executing its code. The runtime version probe passed seven boundary
  cases. These test the extracted Node programs, not Nix evaluation/activation.
- All five imported SDK artifacts matched upstream byte-for-byte; the SDK's MIT
  license is an additional retained file.
- Changed JSON/TOML, Lua, shell, and OpenCode bridge syntax checks passed.
  `git diff --check` passed. Fish syntax and `nix fmt` were not run because those
  executables are unavailable.

The Nix flake/build/Go-test gates above remain unrun: Nix is not installed on
this host. Host-side extension tests and syntax checks do not substitute for
evaluating Home Manager, building the CLI, testing activation, or exercising
live UIs. Paste/gateway services, interactive worktree UI, annotation plugins,
and macOS application preferences have not been exercised live.
