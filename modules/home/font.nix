{ pkgs, lib, ... }: {
  # Iosevka Nerd Font — installed via Homebrew Cask, not Nix.
  # Rationale: macOS resolves fonts through Core Text, which scans
  # ~/Library/Fonts/ and /Library/Fonts/. Brew casks land directly
  # there and the font is available to every macOS app immediately
  # (Ghostty/Kitty/WezTerm/Alacritty, Neovim GUIs, even Word). The
  # Nix-managed nerd-fonts.* derivations only land where fontconfig
  # looks, which is a Linux-first surface; on Darwin they're a square
  # peg.
  #
  # The wizard captures consent in `.dots-state.toml` under
  # capabilities.font; modules/profiles/home.nix imports this file
  # only when that bool is true, so a "No, I already have it"
  # selection short-circuits at the import site and this activation
  # script never runs.
  #
  # Pattern mirrors modules/home/terminals/ghostty.nix: Darwin-only,
  # idempotent (skips when brew already lists the cask), and
  # fail-soft when brew is missing — we surface the install hint
  # rather than panicking the whole HM activation.
  home.activation = lib.mkIf pkgs.stdenv.isDarwin {
    installIosevkaNerdFont = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      if command -v brew >/dev/null 2>&1 && brew list --cask font-iosevka-nerd-font >/dev/null 2>&1; then
        $VERBOSE_ECHO "iosevka-nerd-font: already installed; skipping brew cask"
      elif command -v brew >/dev/null 2>&1; then
        $VERBOSE_ECHO "iosevka-nerd-font: installing via brew cask"
        run brew install --cask font-iosevka-nerd-font
      else
        echo "iosevka-nerd-font: brew not found on PATH — install Homebrew or the font manually." >&2
        echo "         /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"" >&2
      fi
    '';
  };
}
