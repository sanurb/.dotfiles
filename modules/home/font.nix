{ pkgs, ... }: {
  # Iosevka Term Nerd Font — the icon-bearing typeface the terminals
  # (Ghostty/Kitty/WezTerm/Alacritty) and Neovim config assume is on
  # the host. The wizard captures consent in `.dots-state.toml` under
  # capabilities.font; modules/profiles/home.nix imports this file
  # only when that bool is true, so a "No, I already have it"
  # selection short-circuits at the import site and this module's
  # body never evaluates.
  #
  # fontconfig.enable wires HM's per-user font cache so X/Linux apps
  # can see the package's fonts without root. On Darwin, HM symlinks
  # the package into ~/Library/Fonts/HomeManager/ regardless; setting
  # the option there is harmless and keeps the module persona-flat.
  home.packages = [ pkgs.nerd-fonts.iosevka-term ];
  fonts.fontconfig.enable = true;
}
