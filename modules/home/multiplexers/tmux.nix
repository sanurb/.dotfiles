{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
{
  # Install tmux directly via home.packages rather than programs.tmux,
  # because programs.tmux.enable = true authors ~/.config/tmux/tmux.conf
  # itself and would collide with the live-edit symlink below. The
  # imported config/tmux/tmux.conf is rich enough (TPM bootstrap,
  # custom prefix, plugins, theme, copy-mode bindings) that letting
  # HM render its own boilerplate alongside it would be a maintenance
  # trap, not a help.
  home.packages = with pkgs; [ tmux ];

  # Live-editable seam — same pattern as modules/home/editor.nix and
  # modules/home/shells/fish.nix. Single-file symlink (not a dir) so
  # ~/.config/tmux/plugins/ — which TPM populates at runtime — stays
  # outside the symlink chain. Editing config/tmux/tmux.conf in the
  # repo and pressing prefix+r in any tmux session reloads the new
  # config; no `dots apply` round-trip required.
  xdg.configFile."tmux/tmux.conf" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/tmux/tmux.conf";
  };
}
