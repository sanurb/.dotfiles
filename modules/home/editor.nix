{ pkgs, lib, pkgsPins, ... }: {
  programs.neovim = {
    enable = true;
    defaultEditor = true;
    viAlias = true;
    vimAlias = true;

    # Pinned to `pkgsPins.edge` — the editor cycles upstream faster than
    # nixos-unstable's hydra gating, and the lag has bitten plugin compat
    # before. See docs/maintenance.md for the divergence log; collapse this
    # back to `pkgs.neovim` if no divergence is recorded for 90 days.
    package = pkgsPins.edge.neovim;

    extraPackages = with pkgs; [
      ripgrep
      fd
      tree-sitter
    ];

    # Disable remote-provider integrations. With any of withRuby /
    # withPython3 / withNodeJs true, home-manager's neovim wrapper
    # invokes `nvim --headless +UpdateRemotePlugins` during build,
    # which `touch`es `$out/rplugin.vim`. On nixpkgs neovim 0.12.2
    # that step runs after $out is sealed read-only and the build
    # fails with "Permission denied". We do not currently use any
    # remote plugins, so disabling these is both a fix and a
    # principled "Declare, Don't Script" minimum-surface choice.
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
