{
  config,
  pkgs,
  lib,
  ...
}:
let
  # vp self-installs under $VP_HOME and drops its shims in $VP_HOME/bin.
  vpHome = "${config.home.homeDirectory}/.vite-plus";
in
{
  # Vite+ (`vp`) — VoidZero's unified JS toolchain. It ships only as a
  # curl|bash installer (absent from nixpkgs), self-manages under $VP_HOME,
  # and self-updates, so we treat it like the other out-of-Nix package
  # managers in foundation.nix: declare its env/PATH here and let the tool
  # own its bits. The installer also tries to append `. "$VP_HOME/env"` to
  # the shellrc, which no-ops in this setup because HM owns the rc symlinks
  # (read-only /nix/store) — which is precisely why we set PATH ourselves.
  #
  # Kept self-contained (PATH lives here, not in foundation.nix's central
  # list) because this is an opt-out satellite: disabling the module must
  # drop its PATH entry too, which foundation — always-on — could not.
  home.sessionVariables.VP_HOME = vpHome;
  home.sessionPath = [ "${vpHome}/bin" ];

  # Idempotent install hook — same shape as the brew-cask hook in
  # modules/home/terminals/ghostty.nix. Guards on the vp binary so a warm
  # machine is a no-op, and fails soft: a missing prerequisite prints a
  # fix hint rather than aborting activation.
  #
  # `nh home switch` runs with a minimal PATH that lacks the proto shims,
  # so the installer (which shells out to node) wouldn't find node. We
  # prepend proto's shim dirs for the hook and keep VP_NODE_MANAGER=no so
  # vp reuses the proto-pinned node instead of installing a second,
  # competing toolchain under $VP_HOME.
  #
  # This module installs vp and nothing else. pi used to be installed
  # here via `vp install -g`; it now lives in modules/home/pi.nix and is
  # installed with bun, because vp's `packages/<name>#<installId>/`
  # layout breaks pi's extension loader. See the long comment in pi.nix
  # before moving it back.
  home.activation.installVitePlus = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    export PATH="$HOME/.proto/shims:$HOME/.proto/bin:$PATH"

    if [ -x "${vpHome}/bin/vp" ]; then
      $VERBOSE_ECHO "vite-plus: vp already installed; skipping installer"
    else
      $VERBOSE_ECHO "vite-plus: installing vp into ${vpHome}"
      installer="$(mktemp)"
      run ${pkgs.curl}/bin/curl -fsSL https://vite.plus -o "$installer"
      run env VP_HOME="${vpHome}" VP_NODE_MANAGER=no ${pkgs.bash}/bin/bash "$installer"
      rm -f "$installer"
    fi

    if [ ! -x "${vpHome}/bin/vp" ]; then
      echo "vite-plus: vp was NOT installed (installer failed)." >&2
      echo "  fix: run \`curl -fsSL https://vite.plus | bash\` manually, then rerun \`dots apply\`." >&2
    fi
  '';
}
