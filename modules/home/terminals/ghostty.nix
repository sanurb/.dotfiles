{ pkgs, lib, ... }: {
  # Ghostty — config managed as a plain text file so it remains portable
  # to a non-Nix host (just copy the file). Ghostty is a fast-moving
  # project; we don't pin the package here (it's still moving through
  # nixpkgs) — config-only is the conservative option until upstream
  # stabilizes.
  xdg.configFile."ghostty/config".text = ''
    theme = tokyonight
    font-family = JetBrainsMono Nerd Font
    font-size = 14
    window-decoration = false
    cursor-style = block
    shell-integration = detect
    confirm-close-surface = false
  '';
}
