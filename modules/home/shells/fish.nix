{ config, pkgs, lib, workspaceRoot, ... }: {
  programs.fish = {
    enable = true;
    # EDITOR / MANPAGER live in modules/home/editor.nix via
    # home.sessionVariables, which fish picks up through
    # ~/.config/hm-session-vars.fish. nix=nom lives in foundation.nix
    # via home.shellAliases. Only fish-specific behavior stays here.
    interactiveShellInit = ''
      set -g fish_greeting ""

      # Both blocks below run from interactiveShellInit (rendered into
      # ~/.config/fish/config.fish), NOT from a conf.d file, because
      # fish loads conf.d USER→SYS→VENDOR (share/fish/config.fish:216)
      # — vendor conf.d always runs LAST and would clobber any user
      # binding override placed in conf.d. config.fish runs after all
      # conf.d, so this is the only place plugin overrides survive.

      # fzf.fish binding override. Vendor conf.d ends with
      # `fzf_configure_bindings` (no args) which resets to defaults.
      #   alt+c       → directory picker      (mnemonic: change-dir)
      #   ctrl+alt+s  → git status picker
      # Untouched: ctrl+f (accept-autosuggestion), ctrl+r (Atuin).
      if functions -q fzf_configure_bindings
          fzf_configure_bindings --directory=\ec --git_status=\e\cs
      end

      # Puffer mode fix. Puffer's vendor conf.d binds `.`, `!`, `$`,
      # `*` to insert-mode if `fish_key_bindings` isn't exactly
      # `fish_default_key_bindings` — assuming vi mode otherwise.
      # config/fish/conf.d/fish_frozen_key_bindings.fish erases the
      # variable on startup, leaving it empty, so puffer binds to the
      # wrong mode and expansions silently never fire. Setting it here
      # re-triggers puffer's `--on-variable` handler with the right
      # value and rebinds to default mode.
      set fish_key_bindings fish_default_key_bindings
    '';
    shellAbbrs = {
      g = "git";
      ll = "eza -la";
      lt = "eza --tree";
    };
  };

  # Fish plugins — installed as packages so fish auto-loads them via
  # the `$__fish_vendor_*_dir` paths (~/.nix-profile/share/fish/vendor_*.d).
  # programs.fish.plugins isn't usable here: HM resolves it by writing
  # `conf.d/plugin-<name>.fish`, which collides with our directory-level
  # mkOutOfStoreSymlink for fish/conf.d (HM refuses to install files
  # "outside $HOME" when the target is a workspace symlink).
  #
  # - puffer: predictive expansions on plain character keys —
  #     `..` → `../`, `...` → `../../`, `!!` → last command,
  #     `!$` → last arg, `**` → recursive glob. Replaces the manual
  #     `alias up='cd ..'` style of nav. No conflicts with our other
  #     bindings (puffer binds `.`, `!`, `$`, `*`; we use modifier keys).
  #     No tuning required.
  # - fzf.fish: nixpkgs pins `meta.broken = stdenv.hostPlatform.isDarwin`
  #     (pkgs/shells/fish/plugins/fzf-fish.nix:69), but the runtime is
  #     fine — the mark is there because BSD script(1) breaks the
  #     fishtape test runner on macOS, not because the plugin fails.
  #     We override broken=false + doCheck=false; revert when nixpkgs
  #     flips it. Tuning lives in config/fish/conf.d/fzf_config.fish.
  home.packages = [
    pkgs.fishPlugins.puffer
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
