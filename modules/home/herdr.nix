{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
let
  herdrInstallDir = "${config.home.homeDirectory}/.local/bin";
  # The upstream installer checks these commands by name. Supplying their
  # Nix packages explicitly makes installation independent of the host's
  # package set and Home Manager's intentionally minimal activation PATH.
  herdrInstallerPath = lib.makeBinPath [
    pkgs.curl
    pkgs.gawk
    pkgs.coreutils
  ];
in
{
  # herdr — terminal workspace multiplexer with agent-aware panes
  # (herdr.dev). Not in nixpkgs; installed on Linux and macOS with Herdr's
  # official installer from a Home Manager activation script.
  #
  # Only config.toml is seamed. ~/.config/herdr/ doubles as herdr's
  # runtime home (herdr.sock, session.json, plugins/, logs, integration
  # locks) — a whole-dir link would drag live sockets and session state
  # into the repo working tree, the same hazard pi.nix documents for
  # ~/.pi/agent/. Integration shims herdr writes elsewhere
  # (config/opencode/plugins/herdr-agent-state.js and the pi equivalent)
  # are gitignored; regenerate them with `herdr integration`.
  #
  # force=true adopts the config.toml herdr's onboarding wrote before
  # this module owned it.
  xdg.configFile."herdr/config.toml" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/herdr/config.toml";
    force = true;
  };

  # Idempotent cross-platform install. Download the official script to a
  # complete temporary file before executing it rather than piping curl into
  # a shell. The child PATH provides every external command used upstream:
  # curl; awk; and coreutils for uname, sha256sum, mktemp, mkdir, mv, chmod,
  # and cleanup. Herdr's installer verifies the downloaded binary against the
  # SHA-256 digest in its release manifest before moving it into place.
  #
  # Both download and execution are conditions of one `if`. Home Manager
  # activation uses `set -e`, and commands in the condition are exempt from
  # it, preserving the existing fail-soft activation behavior while still
  # printing an actionable recovery command.
  home.activation.installHerdr = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    if command -v herdr >/dev/null 2>&1 || [ -x "${herdrInstallDir}/herdr" ]; then
      $VERBOSE_ECHO "herdr: already installed; skipping installer"
    else
      installer="$(${pkgs.coreutils}/bin/mktemp)"
      $VERBOSE_ECHO "herdr: installing into ${herdrInstallDir}"
      if run ${pkgs.curl}/bin/curl \
        --proto '=https' \
        --tlsv1.2 \
        --fail \
        --silent \
        --show-error \
        --location \
        --retry 3 \
        --connect-timeout 10 \
        --max-time 30 \
        https://herdr.dev/install.sh \
        --output "$installer" \
        && run ${pkgs.coreutils}/bin/env \
          PATH="${herdrInstallerPath}:$PATH" \
          HERDR_INSTALL_DIR="${herdrInstallDir}" \
          ${pkgs.runtimeShell} "$installer"
      then
        $VERBOSE_ECHO "herdr: installer completed"
      fi
      ${pkgs.coreutils}/bin/rm -f "$installer"
    fi

    if [ ! -x "${herdrInstallDir}/herdr" ] && ! command -v herdr >/dev/null 2>&1; then
      echo "herdr: herdr was NOT installed (installer failed)." >&2
      echo "  fix: run \`curl -fsSL https://herdr.dev/install.sh | sh\` manually, then rerun \`dots apply\`." >&2
    fi
  '';
}
