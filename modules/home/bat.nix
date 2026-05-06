{ pkgs, ... }: {
  # bat — persona-agnostic pager. Theme stays in sync with delta
  # (modules/home/delta.nix) so diff output matches `bat` output.
  # bat-extras (batdiff/batman/batgrep/batwatch) ride the same binary;
  # listed under extraPackages so they share the bat closure rather
  # than landing as a parallel home.packages entry.
  programs.bat = {
    enable = true;
    config.theme = "TwoDark";
    extraPackages = with pkgs.bat-extras; [ batdiff batman batgrep batwatch ];
  };
}
