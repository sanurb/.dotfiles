{ pkgs, lib, ... }: {
  programs.alacritty = {
    enable = true;
    settings = {
      font = {
        normal.family = "JetBrainsMono Nerd Font";
        size = 14;
      };
      window = {
        decorations = "None";
        padding = { x = 8; y = 8; };
      };
      cursor.style.shape = "Block";
      bell.duration = 0;
      scrolling.history = 50000;
    };
  };
}
