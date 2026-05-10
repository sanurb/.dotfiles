_: {
  programs.alacritty = {
    enable = true;
    settings = {
      font = {
        normal.family = "Iosevka Nerd Font Mono";
        size = 14;
      };
      window = {
        decorations = "None";
        padding = {
          x = 8;
          y = 8;
        };
      };
      cursor.style.shape = "Block";
      bell.duration = 0;
      scrolling.history = 50000;
    };
  };
}
