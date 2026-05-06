# Neovim follow-ups

Loose ends spotted while wiring up `config/nvim/`. Surfaced here, not
fixed inline.

## 1. No `lazy-lock.json` shipped with the initial config

First launch clones plugins at HEAD; lazy then writes
`lazy-lock.json` into `~/.config/nvim/lazy-lock.json`, which lands at
the in-repo `config/nvim/lazy-lock.json` via the dir-level
mkOutOfStoreSymlink. **Action:** after the first successful `nvim`
launch on a fresh host, `git add config/nvim/lazy-lock.json &&
git commit` to retroactively pin.

## 2. `lualine.lua` shells out to `jj` and `git` on every redraw

`lua/plugins/lualine.lua` calls
`vim.fn.system("jj root 2>/dev/null")` and
`vim.fn.system("git branch --show-current 2>/dev/null")` from the
statusline `cond` callbacks. When `jj` isn't on `$PATH` (most of our
hosts), every redraw spawns a failing subprocess. Output is already
`2>/dev/null`-suppressed, so behavior is correct, only the cost is
wasted.

**Suggested fix:** wrap the calls with
`vim.fn.executable("jj") == 1 and ... or false` once, cache the
result module-locally.

## 3. `mason` still loaded but mostly dormant

`mason-tool-installer`'s auto-run is disabled so the Nix-installed
binaries take precedence. `mason.nvim` itself is still loaded for
ad-hoc `:MasonInstall` use against the small set of fast-moving
tools (`oxlint`, `oxfmt`) that aren't shipped via nixpkgs at the
cadence we want. If nobody reaches for that, drop the dependency
and the `mason-lspconfig` setup call to shave startup time.

## 4. ocamllsp / OCaml stack not installed

`lsp.lua` declares `ocamllsp` with `manual_install = true` plus
`cmd = { "dune", "exec", "ocamllsp" }`. We don't ship dune or
ocaml/opam in the foundation closure. The server entry stays for
users who want it, but unattended `:LspInfo` on a `.ml` file fails
until the user installs OCaml separately.

## 5. `oxlint` / `oxfmt` left to Mason

These move fast and aren't in nixpkgs at the cadence we'd want. They
remain in `ensure_installed` for `mason-tool-installer`, but with
`run_on_start = false` they only land when the user explicitly runs
`:MasonToolsInstall`. Worth re-evaluating once Mason 2.0 lands or
once their packaging stabilises.
