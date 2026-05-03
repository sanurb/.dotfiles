{ pkgs, lib, ... }: {
  # Zsh + Powerlevel10k. P10k owns the prompt; starship integration for
  # zsh is suppressed because two prompt initializers fight and the user
  # picked zsh-with-p10k specifically for that prompt. Starship is still
  # installed (foundation.nix) and active for any non-zsh shells.
  programs.zsh = {
    enable = true;
    enableCompletion = true;
    syntaxHighlighting.enable = true;
    autosuggestion.enable = true;
    historySubstringSearch.enable = true;

    plugins = [
      {
        name = "powerlevel10k";
        src = pkgs.zsh-powerlevel10k;
        file = "share/zsh-powerlevel10k/powerlevel10k.zsh-theme";
      }
    ];

    # p10k instant prompt must be near the top of zshrc. The actual
    # ~/.p10k.zsh tuning file is left to the user to generate via
    # `p10k configure`; we ship the loader, not the opinions.
    initContent = ''
      if [[ -r "''${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-''${(%):-%n}.zsh" ]]; then
        source "''${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-''${(%):-%n}.zsh"
      fi
      [[ -f ~/.p10k.zsh ]] && source ~/.p10k.zsh
    '';

    shellAliases.nix = "nom";
  };

  programs.atuin.enableZshIntegration = true;
  programs.zoxide.enableZshIntegration = true;
  programs.starship.enableZshIntegration = false;
}
