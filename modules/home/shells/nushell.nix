{
  config,
  lib,
  workspaceRoot,
  ...
}:
{
  programs.nushell = {
    enable = true;

    # configFile intentionally left unset — when null, HM does not author
    # ~/.config/nushell/config.nu, leaving the path free for the
    # xdg.configFile symlink below to own. Inlining the config as
    # configFile.text would force a full home-manager rebuild for every
    # tweak; the live-edit symlink avoids that.

    # nix=nom lives in foundation.nix via home.shellAliases (HM
    # propagates it to nushell). Only nushell-specific shell-level
    # rebindings stay here.
    shellAliases = {
      g = "git";
      ll = "eza -la";
    };
  };

  # Live-editable seam — same pattern as modules/home/multiplexers/tmux.nix.
  # Edits to config/nushell/config.nu in the repo are picked up the next
  # time a nushell starts; no `dots apply` round-trip required. When
  # workspaceRoot is empty (HM run outside `dots apply`) we skip the
  # link rather than emit a dangling pointer — nushell falls back to
  # its built-in defaults.
  xdg.configFile."nushell/config.nu" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/nushell/config.nu";
  };

  programs.atuin.enableNushellIntegration = true;
  programs.zoxide.enableNushellIntegration = true;
  programs.starship.enableNushellIntegration = true;
}
