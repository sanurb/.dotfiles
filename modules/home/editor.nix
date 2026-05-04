{ pkgs, lib, pkgsPins, ... }: {
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

    # Minimal base. Plugin orchestration is left to an in-tree
    # lazy.nvim/nixvim layer if/when the user opts in. The option
    # name changed in home-manager: extraLuaConfig → initLua.
    initLua = ''
      vim.opt.number = true
      vim.opt.relativenumber = true
      vim.opt.expandtab = true
      vim.opt.shiftwidth = 2
      vim.opt.tabstop = 2
      vim.opt.termguicolors = true
      vim.opt.signcolumn = "yes"
      vim.opt.undofile = true
      vim.opt.clipboard = "unnamedplus"
      vim.g.mapleader = " "
    '';
  };
}
