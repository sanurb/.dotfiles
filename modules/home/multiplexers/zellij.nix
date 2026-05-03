{ pkgs, lib, ... }: {
  # Zellij — KDL config. Compact layout matches the previous baked-in
  # default; the asset-vs-text decision mirrors ghostty (text-only so
  # it remains portable to a non-Nix host by copy).
  xdg.configFile."zellij/config.kdl".text = ''
    theme "tokyonight"
    default_layout "compact"
    pane_frames false
    copy_command "pbcopy"
    mouse_mode true
    scroll_buffer_size 50000
  '';
}
