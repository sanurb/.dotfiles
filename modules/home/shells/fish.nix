{ config, pkgs, lib, workspaceRoot, ... }: {
  programs.fish = {
    enable = true;
    # EDITOR / MANPAGER live in modules/home/editor.nix via
    # home.sessionVariables, which fish picks up through
    # ~/.config/hm-session-vars.fish. nix=nom lives in foundation.nix
    # via home.shellAliases. Only fish-specific behavior stays here.
    interactiveShellInit = ''
      set -g fish_greeting ""

      # fzf.fish binding override.
      # Lives here, NOT in config/fish/conf.d/, because fish loads conf.d
      # USER→SYS→VENDOR (share/fish/config.fish:216) — so the plugin's
      # vendor_conf.d/fzf.fish (which ends with `fzf_configure_bindings`
      # and no args, resetting every binding to defaults) always runs
      # AFTER any user conf.d. interactiveShellInit gets injected into
      # ~/.config/fish/config.fish, which fish sources after all conf.d
      # processing — so this is the only place a user override survives.
      #
      # Bindings:
      #   alt+c       → directory picker      (mnemonic: change-dir)
      #   ctrl+alt+s  → git status picker
      # Untouched on purpose:
      #   ctrl+f      → fish accept-autosuggestion
      #   ctrl+r      → Atuin (programs.atuin.enableFishIntegration)
      if functions -q fzf_configure_bindings
          fzf_configure_bindings --directory=\ec --git_status=\e\cs
      end
    '';
    shellAbbrs = {
      g = "git";
      ll = "eza -la";
      lt = "eza --tree";
    };
  };

  # fzf.fish — installed as a package so fish auto-loads it via the
  # `$__fish_vendor_*_dir` paths (~/.nix-profile/share/fish/vendor_*.d).
  # We can't use programs.fish.plugins here: HM resolves it by writing
  # `conf.d/plugin-<name>.fish`, which collides with our directory-level
  # mkOutOfStoreSymlink for fish/conf.d (HM refuses to install files
  # "outside $HOME" when the target is a workspace symlink).
  #
  # The nixpkgs derivation pins `meta.broken = stdenv.hostPlatform.isDarwin`
  # (pkgs/shells/fish/plugins/fzf-fish.nix:69) — but the runtime plugin
  # is fine on darwin. The mark is there because the upstream test runner
  # (fishtape via BSD `script(1)`) misbehaves on macOS, not because the
  # plugin itself fails. We override broken=false AND skip checks rather
  # than vendor the source — narrower blast radius, easy to revert when
  # nixpkgs flips it.
  #
  # Tuning (FZF_DEFAULT_OPTS, bindings, previews) lives in
  # config/fish/conf.d/fzf_config.fish so it stays live-editable.
  home.packages = [
    (pkgs.fishPlugins.fzf-fish.overrideAttrs (old: {
      doCheck = false;
      meta = (old.meta or { }) // { broken = false; };
    }))
  ];

  # Foundation integrations — atuin/zoxide/starship are all enabled in
  # foundation.nix; here we just bind them into fish.
  programs.atuin.enableFishIntegration = true;
  programs.zoxide.enableFishIntegration = true;
  programs.starship.enableFishIntegration = true;

  # Live-editable seam — same pattern as modules/home/editor.nix.
  # We symlink each top-level subtree of config/fish/ separately
  # rather than the entire ~/.config/fish directory because
  # programs.fish.enable = true makes HM author
  # ~/.config/fish/config.fish itself (rendered from
  # interactiveShellInit + shellAbbrs + shellAliases above), and a
  # dir-level mkOutOfStoreSymlink would collide with that file. Per-
  # subtree symlinks let HM own config.fish while edits to the
  # imported tree (conf.d entries, custom functions, completions,
  # fisher's plugin file) take effect on the next shell launch
  # without re-running `dots apply`.
  #
  # When workspaceRoot is empty (HM run outside `dots apply`) we skip
  # the link rather than create dangling pointers.
  xdg.configFile = lib.mkIf (workspaceRoot != "") {
    "fish/conf.d".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/conf.d";
    "fish/functions".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/functions";
    "fish/completions".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/completions";
    "fish/fish_plugins".source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/fish/fish_plugins";
  };
}
