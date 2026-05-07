{ config, pkgs, lib, workspaceRoot, ... }: {
  # Ghostty — config managed as a plain text file so it remains portable
  # to a non-Nix host (just copy the file). The upstream `pkgs.ghostty`
  # is Linux-only as of nixpkgs 26.05 (Zig + macOS toolchain mismatch),
  # so on darwin the .app is installed from a HM activation script via
  # Homebrew Cask. This honors the dots promise that selecting a
  # terminal in the wizard installs it — even when the install happens
  # outside Nix on macOS.
  #
  # Live-editable seam — same pattern as modules/home/editor.nix and
  # modules/home/multiplexers/tmux.nix. Editing config/ghostty/config
  # in the repo is picked up the next time ghostty reloads its config;
  # no `dots apply` round-trip required. When workspaceRoot is empty
  # (HM run outside `dots apply`) we skip the link rather than emit a
  # dangling pointer.
  xdg.configFile."ghostty/config" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/ghostty/config";
  };

  # Activation hook: ensure Ghostty.app exists on macOS. Idempotent —
  # checks both /Applications and ~/Applications before invoking brew.
  # The lib.mkIf gate keeps the script out of Linux closures.
  #
  # `nh home switch` doesn't source `brew shellenv`, so PATH at hook
  # time may lack `/opt/homebrew/bin`. We probe the canonical install
  # paths so the hook still works when apply is launched from a shell
  # (or devenv subshell) where shellenv hasn't run.
  home.activation = lib.mkIf pkgs.stdenv.isDarwin {
    installGhostty = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      if [ -d "/Applications/Ghostty.app" ] || [ -d "$HOME/Applications/Ghostty.app" ]; then
        $VERBOSE_ECHO "ghostty: already installed; skipping brew cask"
      else
        BREW=
        for c in $(command -v brew 2>/dev/null) /opt/homebrew/bin/brew /usr/local/bin/brew; do
          [ -x "$c" ] && { BREW=$c; break; }
        done
        if [ -n "$BREW" ]; then
          $VERBOSE_ECHO "ghostty: installing via brew cask ($BREW)"
          run "$BREW" install --cask ghostty
        else
          echo "ghostty: brew not found — Ghostty.app was NOT installed." >&2
          echo "  fix: install Homebrew, then rerun \`dots apply\`:" >&2
          echo "       /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"" >&2
        fi
      fi
    '';
  };
}
