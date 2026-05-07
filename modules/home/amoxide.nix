{ config, pkgs, lib, ... }:
let
  # amoxide isn't in nixpkgs (only `moxide` and `zoxide` exist as
  # near-misses), so we vendor a custom buildRustPackage. Workspace
  # ships two crates: `amoxide` (binary `am`) and `amoxide-tui`
  # (binary `am-tui`). We build the workspace, both binaries land
  # in $out/bin.
  amoxide = pkgs.rustPlatform.buildRustPackage {
    pname = "amoxide";
    version = "0.8.1";

    src = pkgs.fetchFromGitHub {
      owner = "sassman";
      repo = "amoxide-rs";
      rev = "v0.8.1";
      hash = "sha256-Rg3YbW1E95gxdlpKF/4wvzPoi5URARyVKUJxBSPAk/A=";
    };

    cargoHash = "sha256-xMW/9/ISczOzOO/hhjdQH37bI7v3cSB858OFyaN70+Y=";

    # Tests touch real $HOME paths and shell hooks; not worth porting
    # the test harness to the Nix sandbox.
    doCheck = false;

    meta = {
      description = "Shell alias manager — global, profile, or per-project";
      homepage = "https://github.com/sassman/amoxide-rs";
      license = lib.licenses.gpl3Only;
      mainProgram = "am";
    };
  };
in
{
  home.packages = [ amoxide ];

  # amoxide ships a sync hook that reloads aliases on `cd` and a
  # wrapper that re-syncs after `am add`/`am remove`/`am profile use`.
  # The init blob is shell-specific; we wire it via each shell's
  # init seam rather than running `am setup` imperatively (which
  # would patch ~/.zshrc and config.fish — files HM owns).
  programs.fish.interactiveShellInit = lib.mkAfter ''
    if command -q am
        am init fish | source
    end
  '';

  programs.zsh.initContent = lib.mkAfter ''
    if command -v am >/dev/null 2>&1; then
      eval "$(am init zsh)"
    fi
  '';
}
