{
  pkgs,
  lib,
  ...
}:
let
  # tgrep — trigram-indexed grep with a client/server architecture.
  # Pre-builds an on-disk index so queries touch only the small set of
  # files that could match a regex, instead of scanning the whole tree
  # like ripgrep does. Sibling to ast-grep in the "search beyond plain
  # ripgrep" satellite tier; never replaces ripgrep because (a) ripgrep
  # has no server to start and (b) `tgrep` only wins once the index is
  # warm.
  #
  # Not in nixpkgs as of nixos-unstable @ 2026-05-21, so we vendor a
  # buildRustPackage. This follows the amoxide.nix template — same
  # cargoHash discipline, same doCheck rationale — so the maintenance
  # surface stays a single pattern.
  #
  # Workspace ships `tgrep-core` (library) and `tgrep-cli` (binary
  # `tgrep`). buildRustPackage builds the whole workspace; only the
  # binary lands in $out/bin.
  tgrep = pkgs.rustPlatform.buildRustPackage {
    pname = "tgrep";
    version = "0.1.20";

    src = pkgs.fetchFromGitHub {
      owner = "microsoft";
      repo = "tgrep";
      rev = "v0.1.20";
      hash = "sha256-C8YB6QKy+XI+LSY1gwXuF1ZvVUOCIpRPJTwyMPahlmU=";
    };

    cargoHash = "sha256-jmFT9a5jL9kvyNSgJKZUYTxwQz3yxdA1v7DUfk3SEKM=";

    # Tests spawn TCP servers, watch the filesystem via `notify`, and
    # write under $HOME — none of which survive the Nix sandbox. Same
    # call as amoxide.nix.
    doCheck = false;

    meta = {
      description = "Trigram-indexed grep with a client/server architecture for fast regex search in large codebases";
      homepage = "https://github.com/microsoft/tgrep";
      license = lib.licenses.mit;
      mainProgram = "tgrep";
      platforms = lib.platforms.unix;
    };
  };
in
{
  home.packages = [ tgrep ];
}
