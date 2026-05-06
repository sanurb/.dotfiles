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

## 2. Adding fish to `/etc/shells` requires sudo

`internal/loginshell` skips with `SkipNotInEtcShells` when fish is
installed but not registered in `/etc/shells`. The user-facing hint
asks for `sudo tee -a /etc/shells`, but we don't run sudo from
`dots apply`. A future enhancement could either:

- Surface a `dots doctor` finding with a one-liner copy/paste fix.
- Offer a guarded `dots apply --register-shell` opt-in that prompts
  for the password.
