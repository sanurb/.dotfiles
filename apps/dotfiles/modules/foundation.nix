{ pkgs, lib, ... }: {
  # Foundation — the "air" the environment breathes. These three tools
  # (Atuin, Zoxide, Starship) are mandatory infrastructure for every
  # persona, regardless of which shell / terminal / multiplexer the user
  # picked in `dots install`. The TUI never asks about them; home.nix
  # always imports this module.
  #
  # Shell-specific integrations live in the per-shell modules under
  # modules/shells/ — they flip the corresponding `enable<Shell>Integration`
  # bools. We don't enable all of them here because home-manager will try
  # to write into config files for shells that aren't enabled.

  programs.atuin = {
    enable = true;
    settings = {
      auto_sync = false;
      sync_frequency = "0";
      update_check = false;
      style = "compact";
      inline_height = 12;
    };
  };

  programs.zoxide.enable = true;

  # ripgrep — declarative config via home-manager's `arguments` list.
  # Flags here are baked into a generated ripgreprc and exposed via
  # RIPGREP_CONFIG_PATH, so every shell sees the same defaults whether
  # invoked interactively or from scripts.
  programs.ripgrep = {
    enable = true;
    arguments = [
      # Search hidden files / directories.
      "--hidden"
      # Skip really huge binary blobs.
      "--max-filesize=5M"
    ];
  };

  # Starship — config held as a TOML asset and projected as a read-only
  # symlink into ~/.config/starship.toml. Editing the live file is a
  # no-op; the only path to change is editing the asset and re-deploying.
  # Note: when zsh is the chosen shell, modules/shells/zsh.nix sets
  # programs.starship.enableZshIntegration = false so Powerlevel10k owns
  # the prompt — starship still ships its config but doesn't bind.
  programs.starship.enable = true;
  xdg.configFile."starship.toml".source = ../assets/starship.toml;

  # direnv + fzf live here because they're persona-agnostic baseline
  # ergonomics, the same way the previous home.nix kept them at the top.
  home.packages = with pkgs; [ direnv fzf ];
  programs.direnv = {
    enable = true;
    nix-direnv.enable = true;
  };

  # Bash always available as a fallback non-interactive shell, even when
  # the user's interactive persona is fish/zsh/nushell.
  programs.bash = {
    enable = true;
    initExtra = ''
      command -v starship >/dev/null && eval "$(starship init bash)"
      command -v direnv   >/dev/null && eval "$(direnv hook bash)"
    '';
    shellAliases.nix = "nom";
  };
}
