{ config, pkgs, lib, pkgsPins, workspaceRoot, ... }: {
  programs.neovim = {
    enable = true;
    defaultEditor = true;
    viAlias = true;
    vimAlias = true;

    # Pass the *unwrapped* neovim. `programs.neovim.package` expects
    # the bare binary derivation; home-manager's job is to be the
    # wrapper (injecting plugins, providers, the rplugin manifest).
    # Passing the already-wrapped `pkgsPins.edge.neovim` made HM wrap
    # a wrap, and the outer wrap's manifest step `touch`ed
    # rplugin.vim inside the inner wrap's already-sealed $out — the
    # "Permission denied" we'd been chasing.
    #
    # Pinned to `pkgsPins.edge` — the editor cycles upstream faster than
    # nixos-unstable's hydra gating, and the lag has bitten plugin compat
    # before. See docs/maintenance.md for the divergence log; collapse this
    # back to `pkgs.neovim-unwrapped` if no divergence is recorded for 90 days.
    package = pkgsPins.edge.neovim-unwrapped;

    extraPackages = with pkgs; [
      ripgrep
      fd
      tree-sitter
    ];

    # Remote-provider scaffolding off. We don't currently use Python,
    # Ruby, or Node remote plugins; turning these off keeps HM's
    # wrapper from carrying their interpreters into the closure.
    withNodeJs = false;
    withPython3 = false;
    withRuby = false;

    # HM's wrapper bakes only a thin shim that defers to the
    # live-editable user config at $XDG_CONFIG_HOME/nvim/init.lua
    # (symlinked from the workspace by xdg.configFile below). Editing
    # the lua takes effect on the next nvim launch — no flake rebuild,
    # no `dots apply`. fs_stat guards the dofile so a missing or
    # dangling symlink degrades to "no user config" instead of an
    # error at startup.
    initLua = ''
      local user = vim.fn.stdpath("config") .. "/init.lua"
      if vim.uv and vim.uv.fs_stat(user) then dofile(user) end
    '';
  };

  # Live-editable seam. mkOutOfStoreSymlink writes the symlink at
  # activation time; the target path is not copied into the Nix store,
  # so edits to config/nvim/init.lua in the workspace are visible
  # immediately without rebuilding the activation derivation.
  #
  # When workspaceRoot is empty (DOTS_WORKSPACE_ROOT not exported,
  # e.g. someone running home-manager directly outside `dots apply`),
  # we skip the symlink rather than emit a dangling pointer to "/".
  # Nvim falls back to its default empty-config behavior — visibly
  # broken instead of silently wrong.
  xdg.configFile."nvim/init.lua" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/nvim/init.lua";
  };
}
