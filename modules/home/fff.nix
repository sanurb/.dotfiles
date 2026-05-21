{
  pkgs,
  lib,
  ...
}:
let
  # fff — file-search toolkit shipping (a) the `fff-mcp` MCP server,
  # consumed by AI agents (claude-code, opencode, cursor) via
  # config/opencode/opencode.json, and (b) the native cdylib that
  # backs fff.nvim, consumed from config/nvim/lua/plugins/fff.lua.
  #
  # One workspace, one source pin, one buildRustPackage. The Lua
  # plugin tree (lua/, plugin/, doc/) is copied alongside the cdylib
  # under $out/share/fff.nvim/target/release/ so the loader at
  # lua/fff/rust/init.lua resolves the lib via its relative-path
  # search — no env-var indirection inside the lua side.
  #
  # zlob (the Zig-compiled C globbing library) is a workspace
  # *optional* dep: enabled by fff-mcp's `default = ["zlob"]` and by
  # the explicit `zlob` feature on fff-nvim. Dropping defaults with
  # buildNoDefaultFeatures strips zlob entirely, falling back to
  # pure-Rust globset. This avoids pulling Zig 0.16 into the closure
  # (the upstream flake's reason for zig-overlay + crane). The
  # performance cost is bounded to glob matching, not core search.
  fff = pkgs.rustPlatform.buildRustPackage {
    pname = "fff";
    version = "0.8.1";

    src = pkgs.fetchFromGitHub {
      owner = "dmtrKovalenko";
      repo = "fff";
      rev = "v0.8.1";
      hash = "sha256-tA7KR7aj5BmAhfA/IMV4o/FJDO34vXPusDU/Ii+DacU=";
    };

    cargoHash = "sha256-esh3G7+3dXjqLBh8tQnJZaWb0G/nbjeRe3s/5TGctTI=";

    buildNoDefaultFeatures = true;
    cargoBuildFlags = [
      "-p"
      "fff-mcp"
      "-p"
      "fff-nvim"
    ];

    # Tests hit real filesystems, spawn MCP transports, and read
    # $HOME/.config — none of which survive the Nix sandbox.
    doCheck = false;

    postInstall = ''
      # fff-nvim's Cargo.toml declares ~10 dev/bench binaries
      # (test_watcher, jemalloc_profile, grep_profiler, ...) that
      # buildRustPackage's install phase happily ships. We only
      # publish fff-mcp; the rest is build-time scaffolding.
      shopt -s extglob
      rm -f $out/bin/!(fff-mcp)

      # Lay out the nvim plugin tree so its rust/init.lua loader
      # (which searches <plugin>/../../../target/release/lib*.{so,dylib})
      # resolves the cdylib without any env-var inside the lua.
      # lazy.nvim's `dir = ...` points at $out/share/fff.nvim and
      # the relative path lands on target/release/libfff_nvim.*.
      mkdir -p $out/share/fff.nvim/target/release
      cp -r lua plugin doc $out/share/fff.nvim/
      mv $out/lib/libfff_nvim.* $out/share/fff.nvim/target/release/
      rmdir $out/lib
    '';

    meta = {
      description = "File search toolkit — fff-mcp MCP server and fff.nvim native backend";
      homepage = "https://github.com/dmtrKovalenko/fff";
      license = lib.licenses.mit;
      mainProgram = "fff-mcp";
      platforms = lib.platforms.unix;
    };

    # Consumed by config/nvim/lua/plugins/fff.lua via FFF_NVIM_DIR
    # (set below) — keeps the lua spec free of literal store paths.
    passthru.nvimPluginDir = "/share/fff.nvim";
  };
in
{
  home.packages = [ fff ];

  # Bridge the nix-store-resident plugin tree into the user's nvim
  # session. The lazy.nvim spec reads vim.env.FFF_NVIM_DIR and uses
  # it as `dir = ...`. When the var is empty (host where this module
  # is disabled, or no DOTS_WORKSPACE_ROOT), the spec falls back to
  # the upstream GitHub clone + download-binary build hook —
  # graceful degradation rather than a broken plugin.
  home.sessionVariables.FFF_NVIM_DIR = "${fff}${fff.passthru.nvimPluginDir}";
}
