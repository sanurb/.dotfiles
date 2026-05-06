{ config, pkgs, lib, workspaceRoot, ... }: {
  programs.fish = {
    enable = true;
    # EDITOR / MANPAGER live in modules/home/editor.nix via
    # home.sessionVariables, which fish picks up through
    # ~/.config/hm-session-vars.fish. nix=nom lives in foundation.nix
    # via home.shellAliases. Only fish-specific behavior stays here.
    interactiveShellInit = ''
      set -g fish_greeting ""
    '';
    shellAbbrs = {
      g = "git";
      ll = "eza -la";
      lt = "eza --tree";
    };
  };

  # Foundation integrations — atuin/zoxide/starship are all enabled in
  # foundation.nix; here we just bind them into fish.
  programs.atuin.enableFishIntegration = true;
  programs.zoxide.enableFishIntegration = true;
  programs.starship.enableFishIntegration = true;

  # Live-editable seam — same pattern as modules/home/editor.nix.
  # We symlink each top-level subtree of config/fish/ separately
  # rather than the entire ~/.config/fish directory because
  # programs.fish.enable = true makes HM author
  # ~/.config/fish/config.fish itself (rendered from
  # interactiveShellInit + shellAbbrs + shellAliases above), and a
  # dir-level mkOutOfStoreSymlink would collide with that file. Per-
  # subtree symlinks let HM own config.fish while edits to the
  # imported tree (conf.d entries, custom functions, completions,
  # fisher's plugin file) take effect on the next shell launch
  # without re-running `dots apply`.
  #
  # When workspaceRoot is empty (HM run outside `dots apply`) we skip
  # the link rather than create dangling pointers.
  xdg.configFile = lib.mkIf (workspaceRoot != "") {
    "fish/conf.d".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/conf.d";
    "fish/functions".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/functions";
    "fish/completions".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/completions";
    "fish/fish_plugins".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/fish_plugins";
  };
}
