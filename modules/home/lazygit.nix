{ config, pkgs, lib, workspaceRoot, ... }: {
  # Direct package install — NOT programs.lazygit.enable. HM's lazygit
  # module writes a fish helper into ~/.config/fish/functions/lg.fish,
  # which collides with our directory-level mkOutOfStoreSymlink for
  # fish/functions ("outside $HOME" build error — same shape as the
  # fzf-fish issue resolved in modules/home/shells/fish.nix). The
  # package itself ships fish completions in share/fish/vendor_*.d/
  # which fish auto-discovers, so no shell wiring is lost.
  home.packages = [ pkgs.lazygit ];

  xdg.configFile."lazygit/config.yml" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/lazygit/config.yml";
  };
}
