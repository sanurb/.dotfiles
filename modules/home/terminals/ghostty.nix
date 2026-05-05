{ pkgs, lib, ... }: {
  # Ghostty — config managed as a plain text file so it remains portable
  # to a non-Nix host (just copy the file). The upstream `pkgs.ghostty`
  # is Linux-only as of nixpkgs 26.05 (Zig + macOS toolchain mismatch),
  # so on darwin the .app is installed from a HM activation script via
  # Homebrew Cask. This honors the dots promise that selecting a
  # terminal in the wizard installs it — even when the install happens
  # outside Nix on macOS.
  xdg.configFile."ghostty/config".text = ''
    theme = tokyonight
    font-family = JetBrainsMono Nerd Font
    font-size = 14
    window-decoration = false
    cursor-style = block
    shell-integration = detect
    confirm-close-surface = false
  '';

  # Activation hook: ensure Ghostty.app exists on macOS. Idempotent —
  # checks both /Applications and ~/Applications before invoking brew.
  # The lib.mkIf gate keeps the script out of Linux closures.
  home.activation = lib.mkIf pkgs.stdenv.isDarwin {
    installGhostty = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      if [ -d "/Applications/Ghostty.app" ] || [ -d "$HOME/Applications/Ghostty.app" ]; then
        $VERBOSE_ECHO "ghostty: already installed; skipping brew cask"
      elif command -v brew >/dev/null 2>&1; then
        $VERBOSE_ECHO "ghostty: installing via brew cask"
        run brew install --cask ghostty
      else
        echo "ghostty: brew not found on PATH — install Homebrew or Ghostty.app manually." >&2
        echo "         /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"" >&2
      fi
    '';
  };
}
