{ pkgs, ... }:
{
  # Iosevka Nerd Font, sourced from nixpkgs' modular nerd-fonts split
  # (post-2024 reorg — `nerd-fonts.iosevka` adds ~30MB to the closure
  # rather than the ~3GB of the old umbrella `nerdfonts` derivation, so
  # the historical reason to avoid the Nix path no longer applies).
  #
  # Cross-platform by construction:
  #   - Linux: fonts.fontconfig.enable links the package into the HM
  #     profile's share/fonts and runs fc-cache; `fc-list | grep -i
  #     iosevka` resolves post-activation.
  #   - macOS: home-manager's darwin font activation walks share/fonts
  #     in the active profile and symlinks each .ttf/.otf into
  #     ~/Library/Fonts, which Core Text scans. No brew round-trip,
  #     no network during activation, no idempotency book-keeping —
  #     Nix handles that by construction, and flake.lock pins the
  #     font version like every other input.
  #
  # Capability gate (capabilities.font in .dots-state.toml) still lives
  # in modules/profiles/home.nix; opting out short-circuits this module
  # at the import site.
  fonts.fontconfig.enable = true;
  home.packages = [ pkgs.nerd-fonts.iosevka ];
}
