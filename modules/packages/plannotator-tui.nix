{
  lib,
  stdenv,
  stdenvNoCC,
  fetchurl,
  autoPatchelfHook,
}:
let
  # Published SHA256SUMS for v0.6.0, also recorded in
  # https://github.com/plannotator/homebrew-tap/blob/main/Formula/plannotator-tui.rb
  assets = {
    aarch64-darwin = {
      target = "aarch64-apple-darwin";
      sha256 = "096d0c5abda862c173ac7379c606f8909768f43458333ac706bd6ede0424c8b7";
    };
    x86_64-darwin = {
      target = "x86_64-apple-darwin";
      sha256 = "caca14bab235dee26417bfa6b49fe70b7bf6b985af9ea4ad4cd6ba24f56cec31";
    };
    aarch64-linux = {
      target = "aarch64-unknown-linux-gnu";
      sha256 = "4e7391f8a0c815012469675ac69569252eb49cd2b9e9757a5093a6add3a07872";
    };
    x86_64-linux = {
      target = "x86_64-unknown-linux-gnu";
      sha256 = "dfa0e6c0eef9ce1ca64ea7b18f37651364a9f1c2c0a615bdd44f898d2446920a";
    };
  };
  asset = assets.${stdenv.hostPlatform.system};
in
stdenvNoCC.mkDerivation (finalAttrs: {
  pname = "plannotator-tui";
  version = "0.6.0";
  src = fetchurl {
    url = "https://github.com/plannotator/plannotator-tui/releases/download/v${finalAttrs.version}/plannotator-tui-${asset.target}";
    inherit (asset) sha256;
  };
  dontUnpack = true;
  nativeBuildInputs = lib.optionals stdenv.hostPlatform.isLinux [ autoPatchelfHook ];
  buildInputs = lib.optionals stdenv.hostPlatform.isLinux [ stdenv.cc.cc.lib ];
  installPhase = ''
    runHook preInstall
    install -Dm755 "$src" "$out/bin/plannotator-tui"
    runHook postInstall
  '';
  doInstallCheck = true;
  installCheckPhase = ''
    runHook preInstallCheck
    "$out/bin/plannotator-tui" --version | grep -F '${finalAttrs.version}'
    runHook postInstallCheck
  '';
  meta = {
    description = "Annotate Markdown in the terminal and send reviews to coding agents";
    homepage = "https://github.com/plannotator/plannotator-tui";
    license = lib.licenses.mit;
    platforms = builtins.attrNames assets;
    mainProgram = "plannotator-tui";
  };
})
