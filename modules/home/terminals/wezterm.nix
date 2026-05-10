{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
{
  # Install wezterm directly via home.packages rather than programs.wezterm,
  # because programs.wezterm.enable = true authors ~/.config/wezterm/wezterm.lua
  # itself (rendered from extraConfig) and would collide with the live-edit
  # symlink below. Inlining the lua as an extraConfig string forces a full
  # home-manager rebuild on every keystroke; the symlink avoids that.
  home.packages = with pkgs; [ wezterm ];

  # Live-editable seam — same pattern as modules/home/editor.nix and
  # modules/home/multiplexers/tmux.nix. Single-file symlink so anything
  # wezterm writes alongside the config (state, cache) stays outside
  # the symlink chain.
  xdg.configFile."wezterm/wezterm.lua" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/wezterm/wezterm.lua";
  };
}
