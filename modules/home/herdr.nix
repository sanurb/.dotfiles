{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
{
  # herdr — terminal workspace multiplexer with agent-aware panes
  # (herdr.dev). Not in nixpkgs; installed via Homebrew from a HM
  # activation script, same shape as modules/home/terminals/ghostty.nix.
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

  # Idempotent install. herdr's curl installer drops the binary in
  # ~/.local/bin, which may not be on PATH during activation, so probe
  # that location explicitly before falling back to brew.
  home.activation = lib.mkIf pkgs.stdenv.isDarwin {
    installHerdr = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      if command -v herdr >/dev/null 2>&1 || [ -x "$HOME/.local/bin/herdr" ]; then
        $VERBOSE_ECHO "herdr: already installed; skipping brew"
      else
        BREW=
        for c in $(command -v brew 2>/dev/null) /opt/homebrew/bin/brew /usr/local/bin/brew; do
          [ -x "$c" ] && { BREW=$c; break; }
        done
        if [ -n "$BREW" ]; then
          $VERBOSE_ECHO "herdr: installing via brew ($BREW)"
          run "$BREW" install herdr
        else
          echo "herdr: brew not found — herdr was NOT installed." >&2
          echo "  fix: install Homebrew, then rerun \`dots apply\`." >&2
        fi
      fi
    '';
  };
}
