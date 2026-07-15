# Fish follow-ups

Loose ends from wiring up `config/fish/` and the wizard→login-shell
fix. Surfaced here, not fixed inline.

## 1. Pre-existing `homeModules.font` reference

`modules/profiles/home.nix` imports `homeModules.font` and a
matching `font` capability. The `modules/home/font.nix` file did not
yet exist when the fish work landed, so the flake could not evaluate
on a clean working tree. Resolved separately when `font.nix` was
committed; left here in case the same shape recurs:

```
error: attribute 'font' missing
       at modules/profiles/home.nix:
        ++ lib.optional (caps.font or true) homeModules.font;
                                            ^
```

The lesson: any new pillar/capability under `caps.<name>` must land
the matching `modules/home/<name>.nix` in the same commit.

## 2. Adding fish to `/etc/shells` requires sudo — RESOLVED

`internal/loginshell` now returns `RegisterShell` (was
`SkipNotInEtcShells`) when the selected shell is installed but not
registered in `/etc/shells`. `Apply` acts on it instead of only
hinting:

- **Interactive `dots apply`** — prompts for consent (defaults to yes,
  since the shell was chosen in the wizard), then `sudo`-appends the
  path to `/etc/shells` (idempotent `grep -qxF` guard) and runs
  `chsh -s`. One password prompt finishes the switch, self-service.
- **Headless / stream path** — no TTY, so it degrades to the copy/paste
  one-liner and never blocks. Same for `DOTS_REGISTER_SHELL=never`.
- **Escape hatches** — `DOTS_YES=1` or `DOTS_REGISTER_SHELL=always`
  skip the y/n prompt (sudo still gates); `DOTS_REGISTER_SHELL=never`
  opts out entirely.
- **`dots doctor`** now carries a `login shell` finding: `SevPass`
  when active, `SevWarn` with the exact fix when installed-but-inactive.

The target is `~/.nix-profile/bin/<shell>` — the per-user profile is a
GC root, so the current generation's binary is retained by
`nix-collect-garbage`; that path is the stable choice over a bare store
path (which is not itself a root).
