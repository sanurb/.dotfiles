{ pkgs, lib, ... }: {
  programs.neovim = {
    enable = true;
    defaultEditor = true;
    viAlias = true;
    vimAlias = true;

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
