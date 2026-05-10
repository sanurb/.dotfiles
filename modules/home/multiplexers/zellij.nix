{
  config,
  lib,
  workspaceRoot,
  ...
}:
{
  # Zellij — KDL config. Live-editable seam, same pattern as
  # modules/home/multiplexers/tmux.nix. Edits to config/zellij/config.kdl
  # are picked up the next time zellij is launched; no `dots apply`
  # round-trip required. When workspaceRoot is empty (HM run outside
  # `dots apply`) we skip the link rather than emit a dangling pointer.
  xdg.configFile."zellij/config.kdl" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/zellij/config.kdl";
  };
}
