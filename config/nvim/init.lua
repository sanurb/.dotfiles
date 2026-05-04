-- Live-edit surface. Symlinked to ~/.config/nvim/init.lua via
-- programs.neovim's xdg.configFile entry in modules/home/editor.nix.
-- Edits take effect on the next nvim launch — no flake rebuild,
-- no `dots apply`. The home-manager module owns the wrapper, the
-- plugin closure, and provider scaffolding; this file owns runtime
-- behavior.

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
