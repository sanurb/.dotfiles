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

    # Tools nvim invokes from inside its own session (LSP `cmd` arrays,
    # treesitter parser builds, telescope's rg/fd backends). These land
    # on the wrapped nvim's PATH but stay out of the user's interactive
    # PATH — keeps `which gopls` honest about the editor environment
    # vs. the shell environment.
    extraPackages = with pkgs; [
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
      vscode-langservers-extracted # provides eslint, cssls, html, jsonls
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

    # Remote-provider scaffolding off. We don't currently use Python,
    # Ruby, or Node remote plugins; turning these off keeps HM's
    # wrapper from carrying their interpreters into the closure and
    # sidesteps the rplugin.vim manifest-rewrite race that bit us in
    # the v0.4.x activation saga.
    withNodeJs = false;
    withPython3 = false;
    withRuby = false;

    # No initLua / extraConfig — leaving both empty makes HM author no
    # init.lua of its own, so nvim picks up the dir-level
    # mkOutOfStoreSymlink at xdg.configFile."nvim" below as the single
    # source of truth. Setting initLua here would have HM emit
    # ~/.config/nvim/init.lua into the Nix store, colliding with the
    # symlink target.
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
