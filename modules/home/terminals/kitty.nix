{ pkgs, lib, ... }:
{
  programs.kitty = {
    enable = true;
    themeFile = "tokyo_night_night";
    font = {
      name = "Iosevka Nerd Font Mono";
      size = 14;
    };
    settings = {
      confirm_os_window_close = 0;
      cursor_shape = "block";
      enable_audio_bell = false;
      hide_window_decorations = "yes";
      shell_integration = "enabled";
      tab_bar_style = "powerline";
    };
  };
}
