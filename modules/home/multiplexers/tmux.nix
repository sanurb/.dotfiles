{ pkgs, lib, ... }: {
  programs.tmux = {
    enable = true;
    baseIndex = 1;
    keyMode = "vi";
    mouse = true;
    prefix = "C-a";
    terminal = "tmux-256color";
    historyLimit = 50000;
    escapeTime = 10;

    extraConfig = ''
      set -ga terminal-overrides ",*256col*:Tc"
      set -g focus-events on
      set -g renumber-windows on
      set -g status-position top

      # Splits inherit cwd
      bind '"' split-window -v -c "#{pane_current_path}"
      bind %   split-window -h -c "#{pane_current_path}"

      # Reload config
      bind r source-file ~/.config/tmux/tmux.conf \; display "tmux.conf reloaded"
    '';
  };
}
