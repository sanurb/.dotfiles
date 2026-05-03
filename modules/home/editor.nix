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

    # Minimal base. Plugin orchestration is left to an in-tree
    # lazy.nvim/nixvim layer if/when the user opts in.
    extraLuaConfig = ''
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
