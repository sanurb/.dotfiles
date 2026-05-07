{ pkgs, lib, inputs, system, ... }:
let
  envUser = builtins.getEnv "USER";
  envHome = builtins.getEnv "HOME";
  # Workspace path. `dots apply` exports DOTS_WORKSPACE_ROOT into
  # nh's env (apps/cli/internal/activation/env.go). Empty when run
  # outside `dots apply` — modules that depend on it gate their
  # output (mkOutOfStoreSymlink) so a missing workspace produces no
  # symlink rather than a dangling pointer to "/".
  envWS = builtins.getEnv "DOTS_WORKSPACE_ROOT";
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
  # path via envWS (DOTS_WORKSPACE_ROOT) rather than a path literal
  # because the file is gitignored — flake source filtering would hide
  # a path-literal lookup. With --impure (already in use for the build),
  # readFile against an absolute string path bypasses the source filter
  # cleanly.
  # Schema version is declared once at apps/cli/internal/state/SCHEMA_VERSION
  # and read from there by both Go (go:embed) and Nix (this readFile). Drift
  # between the two consumers is impossible by construction — there is no
  # "Nix-side constant" to forget to bump.
  schemaVersion = lib.toInt (lib.removeSuffix "\n"
    (builtins.readFile ../../apps/cli/internal/state/SCHEMA_VERSION));

  defaultState = {
    schema_version = schemaVersion;
    pillars = { shell = "fish"; terminal = "ghostty"; multiplexer = "zellij"; };
    capabilities = { editor = true; font = true; };
    modules = { bat = true; delta = true; gh = true; lazygit = true; opencode = true; };
  };
  stateFile = if envWS != "" then "${envWS}/.dots-state.toml" else "";
  state =
    if stateFile != "" && builtins.pathExists stateFile
    then builtins.fromTOML (builtins.readFile stateFile)
    else defaultState;

  pillars = state.pillars or defaultState.pillars;
  caps = state.capabilities or defaultState.capabilities;
  # Satellite modules — schema v2. v1 state files have no [modules]
  # section; `state.modules or {}` returns an empty attrset and each
  # satellite then resolves to its `or true` default below, so a v1
  # file boots with all satellites enabled (behavior-preserving).
  modules = state.modules or { };

  # Module discovery — the directory tree under ../home/ is the registry.
  # `homeModules.shells.fish` resolves to ../home/shells/fish.nix, and a
  # new shell module appears here automatically the moment its file is
  # created. Adding/removing modules touches no central import list.
  homeModules = (import ../lib/discover.nix { inherit lib; }).discoverDir ../home;

  # Pillar resolution — defensive `or` falls through to a sane default
  # rather than throwing on an unrecognized state value. Validation
  # lives in the CLI (state.Validate); this is the second line of defense.
  shellModule = homeModules.shells.${pillars.shell}        or homeModules.shells.fish;
  terminalModule = homeModules.terminals.${pillars.terminal}  or homeModules.terminals.ghostty;
  multiplexerModule = homeModules.multiplexers.${pillars.multiplexer} or null;
in
{
  imports =
    # Foundation + git are mandatory infrastructure (identity is sourced
    # externally per modules/home/git.nix); the wizard never asks about
    # them. Editor and font remain user-toggleable capabilities.
    # Satellite tools (bat, delta, gh, lazygit, opencode) default-true
    # via `modules.<name> or true`, preserving v1 behavior on hosts
    # whose state file predates the [modules] section.
    [
      homeModules.foundation
      homeModules.git
      shellModule
      terminalModule
    ]
    ++ lib.optional (multiplexerModule != null) multiplexerModule
    ++ lib.optional (caps.editor   or true) homeModules.editor
    ++ lib.optional (caps.font     or true) homeModules.font
    ++ lib.optional (modules.bat      or true) homeModules.bat
    ++ lib.optional (modules.delta    or true) homeModules.delta
    ++ lib.optional (modules.gh       or true) homeModules.gh
    ++ lib.optional (modules.lazygit  or true) homeModules.lazygit
    ++ lib.optional (modules.opencode or true) homeModules.opencode;

  home.username = if envUser != "" then envUser else "dots";
  home.homeDirectory = if envHome != "" then envHome else fallbackHome;
  home.stateVersion = "26.05";

  programs.home-manager.enable = true;

  _module.args.identity = identity;
  # Modules that emit live-edit symlinks (mkOutOfStoreSymlink targets)
  # consume workspaceRoot rather than calling builtins.getEnv themselves
  # — one definition, one consumer, easy to grep when adding the next
  # live-editable surface.
  _module.args.workspaceRoot = envWS;
}
