{ pkgs, lib, inputs, system, ... }:
let
  envUser = builtins.getEnv "USER";
  envHome = builtins.getEnv "HOME";
  envWS   = builtins.getEnv "MOON_WORKSPACE_ROOT";
  isDarwin = pkgs.stdenv.isDarwin;
  fallbackHome = if isDarwin then "/Users/dots" else "/home/dots";

  # Identity boundary — name/email come from a host-local file that is
  # gitignored. If absent, identity stays empty — commits fail loudly,
  # which is the correct failure mode for a public-safe repo.
  identityPath = "${envHome}/.config/dots/identity.nix";
  identity =
    if envHome != "" && builtins.pathExists identityPath
    then import identityPath
    else { name = ""; email = ""; };

  # Persona — the dots install TUI writes .dots-state.toml at the
  # workspace root; that file is the single source of truth for which
  # shell, terminal, and multiplexer this host gets. We resolve the
  # path via $MOON_WORKSPACE_ROOT (already set by `moon run`) rather
  # than a path literal because the file is gitignored — flake source
  # filtering would hide a path-literal lookup. With --impure (already
  # in use for the build), readFile against an absolute string path
  # bypasses the source filter cleanly.
  defaultState = {
    schema_version = 1;
    pillars      = { shell = "fish"; terminal = "ghostty"; multiplexer = "zellij"; };
    capabilities = { editor = true; git = true; };
  };
  stateFile = if envWS != "" then "${envWS}/.dots-state.toml" else "";
  state =
    if stateFile != "" && builtins.pathExists stateFile
    then builtins.fromTOML (builtins.readFile stateFile)
    else defaultState;

  pillars = state.pillars or defaultState.pillars;
  caps    = state.capabilities or defaultState.capabilities;

  # Pillar resolution — explicit if/else over a `${attrset.${key}}`
  # subscript so an unrecognized state value falls through to a sane
  # default rather than throwing during eval. Validation lives in the
  # CLI (state.Validate); this is the second line of defense.
  shellModule =
    if pillars.shell == "zsh"     then ../../modules/home/shells/zsh.nix
    else if pillars.shell == "nushell" then ../../modules/home/shells/nushell.nix
    else ../../modules/home/shells/fish.nix;

  terminalModule =
    if pillars.terminal == "kitty"     then ../../modules/home/terminals/kitty.nix
    else if pillars.terminal == "wezterm"   then ../../modules/home/terminals/wezterm.nix
    else if pillars.terminal == "alacritty" then ../../modules/home/terminals/alacritty.nix
    else ../../modules/home/terminals/ghostty.nix;

  multiplexerModule =
    if pillars.multiplexer == "tmux" then ../../modules/home/multiplexers/tmux.nix
    else if pillars.multiplexer == "zellij" then ../../modules/home/multiplexers/zellij.nix
    else null;  # "none" — no multiplexer module imported.
in {
  imports =
    [ ../../modules/home/foundation.nix shellModule terminalModule ]
    ++ lib.optional (multiplexerModule != null) multiplexerModule
    ++ lib.optional (caps.editor or true) ../../modules/home/editor.nix
    ++ lib.optional (caps.git    or true) ../../modules/home/git.nix;

  home.username      = if envUser != "" then envUser else "dots";
  home.homeDirectory = if envHome != "" then envHome else fallbackHome;
  home.stateVersion  = "26.05";

  programs.home-manager.enable = true;

  _module.args.identity = identity;
}
