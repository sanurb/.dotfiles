{ pkgs, lib, ... }: {
  programs.fish = {
    enable = true;
    interactiveShellInit = ''
      set -g fish_greeting ""
    '';
    shellAbbrs = {
      g  = "git";
      ll = "eza -la";
      lt = "eza --tree";
    };
    # Route nix through nom (nix-output-monitor). nom is a transparent
    # drop-in: it forwards any subcommand to nix and renders a live
    # build graph for the build/develop/shell/run paths.
    shellAliases.nix = "nom";
  };

  # Foundation integrations — atuin/zoxide/starship are all enabled in
  # foundation.nix; here we just bind them into fish.
  programs.atuin.enableFishIntegration    = true;
  programs.zoxide.enableFishIntegration   = true;
  programs.starship.enableFishIntegration = true;
}
