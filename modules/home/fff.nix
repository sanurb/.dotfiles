{
  pkgs,
  lib,
  ...
}:
let
  # zlob is workspace-optional and only fff-mcp's default features
  # enable it. buildNoDefaultFeatures keeps Zig 0.16 out of the closure
  # (the reason upstream's flake otherwise needs zig-overlay + crane).
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
    # Without --bin/--lib filters, `-p fff-nvim` builds ~11 bench and
    # profiler binaries declared in its Cargo.toml — each a fat-LTO
    # final link we'd then have to delete. Filter to only what ships.
    cargoBuildFlags = [
      "--package"
      "fff-mcp"
      "--bin"
      "fff-mcp"
      "--package"
      "fff-nvim"
      "--lib"
    ];

    # Tests spawn MCP transports and write $HOME — not sandbox-safe.
    doCheck = false;

    # lua/fff/rust/init.lua hard-codes <plugin>/../../../target/release
    # as the cdylib search path, so lazy.nvim's `dir = $out/share/fff.nvim`
    # only resolves the lib if it lands at that exact relative offset.
    postInstall = ''
      mkdir -p $out/share/fff.nvim/target/release
      cp -r lua plugin doc $out/share/fff.nvim/
      mv $out/lib/libfff_nvim.* $out/share/fff.nvim/target/release/
      rm -rf $out/lib
    '';

    meta = {
      description = "File search toolkit — fff-mcp MCP server and fff.nvim native backend";
      homepage = "https://github.com/dmtrKovalenko/fff";
      license = lib.licenses.mit;
      mainProgram = "fff-mcp";
      platforms = lib.platforms.unix;
    };
  };
in
{
  home.packages = [ fff ];

  # Empty on hosts where this module is disabled — fff.lua nil-checks
  # and falls back to lazy.nvim's upstream clone + build hook.
  home.sessionVariables.FFF_NVIM_DIR = "${fff}/share/fff.nvim";
}
