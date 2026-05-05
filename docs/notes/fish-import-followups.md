# Fish-import follow-ups

These came up while landing the dmmulroy fish import + wizard-fish wiring
fixes; they're surfaced here rather than mixed into the same session.

## 1. Pre-existing `homeModules.font` reference is broken

`modules/profiles/home.nix` (uncommitted local WIP at the time of the
fish import) imports `homeModules.font` and a corresponding `font`
capability, but `modules/home/font.nix` is untracked. With a clean git
working tree, the flake cannot evaluate (`nix build .#homeActivation`
errors with "attribute 'font' missing"). Either commit `modules/home/font.nix`
or revert the `home.nix` reference. **Not fixed in this session because
it's WIP from a different feature branch.**

Symptom (verbatim):

```
error: attribute 'font' missing
       at modules/profiles/home.nix:71:
        ++ lib.optional (caps.font or true) homeModules.font;
                                            ^
```

## 2. fish module overrides whole `~/.config/fish` directory

`modules/home/shells/fish.nix` now sets
`xdg.configFile."fish".source = mkOutOfStoreSymlink ".../config/fish"`.
That replaces the entire HM-managed fish config directory, including
the auto-generated `conf.d/hm-session-vars.fish` that bridges
`home.sessionPath` into fish's $PATH.

Mitigation already in place: `config/fish/conf.d/path.fish` mirrors the
`home.sessionPath` list and adds the missing dirs at fish startup.

Drawback: `programs.fish.shellAbbrs` and `programs.fish.shellAliases`
declared in `fish.nix` are silently dropped because HM's generated
`config.fish` is shadowed by the symlink. The current abbrs (`g`, `ll`,
`lt`) and the `nix=nom` alias need to be migrated into
`config/fish/conf.d/` if they are still wanted. **Not fixed here**: the
import session was scoped to the dmmulroy tree + wiring fixes.

## 3. Adding fish to `/etc/shells` requires sudo

`internal/loginshell` skips with `SkipNotInEtcShells` when fish is
installed but not registered in `/etc/shells`. The user-facing hint
asks for `sudo tee -a /etc/shells`, but we don't run sudo from
`dots apply`. A future enhancement could either:

- Surface a `dots doctor` finding with a one-liner copy/paste fix.
- Offer a guarded `dots apply --register-shell` opt-in that prompts
  for the password.
