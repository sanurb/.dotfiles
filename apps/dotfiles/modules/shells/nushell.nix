{ pkgs, lib, ... }: {
  programs.nushell = {
    enable = true;

    # Nushell config — keep it minimal; the user can layer on top via
    # ~/.config/nushell/env.nu and config.nu (which we leave alone).
    configFile.text = ''
      $env.config = {
        show_banner: false
        edit_mode: emacs
        history: { max_size: 100000, sync_on_enter: true, file_format: "sqlite" }
        completions: { case_sensitive: false, quick: true, partial: true }
      }
    '';

    shellAliases = {
      nix = "nom";
      g   = "git";
      ll  = "eza -la";
    };
  };

  programs.atuin.enableNushellIntegration    = true;
  programs.zoxide.enableNushellIntegration   = true;
  programs.starship.enableNushellIntegration = true;
}
