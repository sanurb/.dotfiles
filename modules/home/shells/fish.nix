{ config, pkgs, lib, workspaceRoot, ... }: {
  programs.fish = {
    enable = true;
    interactiveShellInit = ''
      set -g fish_greeting ""
    '';
    shellAbbrs = {
      g = "git";
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
  programs.atuin.enableFishIntegration = true;
  programs.zoxide.enableFishIntegration = true;
  programs.starship.enableFishIntegration = true;

  # Live-editable seam — same pattern as modules/home/editor.nix.
  # Pointing ~/.config/fish at the in-repo tree means edits to
  # conf.d/aliases.fish, functions/<f>.fish, etc. take effect on the
  # next shell launch without re-running `dots apply`. fish itself
  # autoloads from this directory at every startup, so the live-edit
  # ergonomics fall out for free.
  #
  # mkOutOfStoreSymlink keeps the target out of the Nix store; when
  # workspaceRoot is empty (HM run outside `dots apply`) we skip the
  # link rather than create a dangling pointer to "/config/fish".
  xdg.configFile."fish" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish";
  };
}
