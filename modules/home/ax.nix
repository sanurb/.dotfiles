{
  pkgs,
  lib,
  ...
}:
let
  # ax isn't in nixpkgs. Upstream ships a flake, but its package.nix
  # builds the TypeScript sources through bun2nix, so consuming it would
  # mean adding both that flake and bun2nix as inputs — a second nixpkgs
  # evaluation on every apply, plus a source build of a program upstream
  # already publishes as a signed single-file binary.
  #
  # We fetch the release asset and pin the SHA-256 that upstream's own
  # checksums.txt publishes for v0.1.25, which is the same digest its
  # install script verifies. Same shape as modules/home/amoxide.nix: a
  # vendored derivation rather than a flake input.
  assets = {
    aarch64-darwin = {
      asset = "ax-darwin-arm64";
      sha256 = "94633f9d61c743cffcf51e5338289d974c21428efb70bd2995e6c6ec13e062c1";
    };
    x86_64-darwin = {
      asset = "ax-darwin-x64";
      sha256 = "c33b0a72df9960116e8467bfa0b170eec3f57244630ec8018efc593c7c375083";
    };
    aarch64-linux = {
      asset = "ax-linux-arm64";
      sha256 = "631fabf91071b30022039edb0c89a5e8768067cc00c8cf7ddb598326f02d2d5c";
    };
    x86_64-linux = {
      asset = "ax-linux-x64";
      sha256 = "6e72297182074038a202d94e1cca9d61e8908d8a4fd5206b3c25ab91957cae2f";
    };
  };

  ax = pkgs.stdenvNoCC.mkDerivation (finalAttrs: {
    pname = "ax";
    version = "0.1.25";

    src =
      let
        a =
          assets.${pkgs.stdenv.hostPlatform.system}
            or (throw "ax: no release asset for ${pkgs.stdenv.hostPlatform.system}");
      in
      pkgs.fetchurl {
        url = "https://github.com/yusukebe/ax/releases/download/v${finalAttrs.version}/${a.asset}";
        inherit (a) sha256;
      };

    dontUnpack = true;
    # Stripping removes the embedded application payload and leaves only Bun.
    dontStrip = true;

    # The asset is a Bun single-file executable: self-contained JS runtime
    # plus the bundled linkedom parser, so there is no node/bun to provide
    # at runtime. On Linux it still links against glibc and needs its
    # interpreter rewritten for the Nix store.
    nativeBuildInputs = lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.autoPatchelfHook ];
    buildInputs = lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.stdenv.cc.cc.lib ];

    installPhase = ''
      runHook preInstall
      install -Dm755 "$src" "$out/bin/ax"
      runHook postInstall
    '';

    doInstallCheck = true;
    installCheckPhase = ''
      runHook preInstallCheck
      reported=$("$out/bin/ax" --version 2>&1)
      case "$reported" in
        *'${finalAttrs.version}'*) ;;
        *)
          echo "ax --version reported '$reported', expected ${finalAttrs.version}" >&2
          exit 1
          ;;
      esac
      runHook postInstallCheck
    '';

    meta = {
      description = "Fetch, discover, and extract structured data from web pages in one command";
      homepage = "https://github.com/yusukebe/ax";
      license = lib.licenses.mit;
      platforms = builtins.attrNames assets;
      mainProgram = "ax";
    };
  });
in
{
  # No shell integration: ax is a plain binary that reads argv and writes
  # stdout, which is the whole point of it being agent-facing.
  home.packages = [ ax ];
}
