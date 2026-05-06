{ config, pkgs, lib, pkgsPins, workspaceRoot, ... }: {
  # Editor — wrapper-free path. We install neovim plus its runtime
  # toolchain via home.packages and point ~/.config/nvim at the live
  # workspace tree via a single out-of-store symlink. No
  # `programs.neovim` block on purpose: HM's wrapper unconditionally
  # authors ~/.config/nvim/init.lua (its luaRcContent is non-empty as
  # soon as viAlias / package / withNodeJs / etc. are set), and that
  # per-file rule collides with the directory symlink below — HM walks
  # the file, its `realpath -m` follows the dir symlink off-tree, and
  # home-manager-files bails with "Error installing file ... outside
  # $HOME". See nix-community/home-manager modules/files.nix line ~398
  # for the realpath-stays-under-output check.
  #
  # Trade-off: LSPs / formatters / rg / fd land on the user's
  # interactive PATH instead of being wrapper-private. In practice
  # nvim invokes them by name and so does the shell, so the
  # difference is cosmetic.
  home.packages = with pkgs; [
    # Neovim itself — pinned to pkgsPins.edge for plugin compat with
    # lazy.nvim's HEAD ecosystem (nixos-unstable's hydra gating lags).
    # Wrapped variant on purpose; outside `programs.neovim.package`
    # there's no double-wrap risk, and the wrapper's
    # `--cmd "lua vim.g.loaded_{node,perl,ruby,python3}_provider=0"`
    # is the modern equivalent of the old `withNodeJs/withPython3/
    # withRuby = false` flags — providers stay out of the closure.
    pkgsPins.edge.neovim

    # Telescope / oil / treesitter externals
    ripgrep
    fd
    tree-sitter

    # LSPs from nixpkgs — every server with a stable package goes
    # here. The handful that remain on Mason (oxlint, oxfmt) don't
    # have nixpkgs derivations at the cadence we need; see
    # config/nvim/lua/plugins/lsp.lua's `nix_provided` map for the
    # boundary.
    bash-language-server
    biome
    vscode-langservers-extracted # eslint, cssls, html, jsonls
    lua-language-server
    marksman
    sqls
    tailwindcss-language-server
    yaml-language-server
    zls
    svelte-language-server
    rust-analyzer

    # Formatters
    prettierd
    stylua
  ];

  # Default editor + man-page reader — what `programs.neovim.defaultEditor
  # = true` used to wire (only EDITOR; MANPAGER is added explicitly).
  # home.sessionVariables writes ~/.config/hm-session-vars.{sh,fish}
  # which every HM-managed shell sources, so $EDITOR / $MANPAGER
  # resolve to nvim in git, less, sudoedit, man, etc. without
  # duplicating the binding in each shell's interactiveShellInit.
  home.sessionVariables = {
    EDITOR = "nvim";
    MANPAGER = "nvim +Man!";
  };

  # vi / vim aliases — replaces `programs.neovim.{viAlias,vimAlias}`.
  # home.shellAliases propagates to every enabled shell module
  # (bash/zsh/fish) but only fires in interactive shells, so
  # non-interactive scripts running bare `vi` (cron, sudoedit) get
  # system vi by design — POSIX semantics preserved.
  home.shellAliases = {
    vi = "nvim";
    vim = "nvim";
  };

  # Live-editable seam. The whole config/nvim/ directory becomes
  # ~/.config/nvim/ via a single out-of-store symlink, so edits to
  # any lua file (init.lua, lua/dots/*.lua, lua/plugins/*.lua,
  # after/**) are picked up on the next nvim launch — no flake
  # rebuild, no `dots apply`. lazy.nvim writes its lazy-lock.json
  # back into the same tree, which is why we want the directory
  # symlink and not per-subtree links: the lockfile lives at the
  # top level alongside init.lua.
  #
  # When workspaceRoot is empty (DOTS_WORKSPACE_ROOT not exported,
  # e.g. someone running home-manager directly outside `dots apply`),
  # we skip the symlink rather than emit a dangling pointer to "/".
  # Nvim falls back to its default empty-config behavior — visibly
  # broken instead of silently wrong.
  xdg.configFile."nvim" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/nvim";
  };
}
