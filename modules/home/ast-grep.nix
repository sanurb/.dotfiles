{ pkgs, ... }:
{
  # ast-grep — structural code search via tree-sitter ASTs. Sibling to
  # ripgrep (already in modules/home/editor.nix) for cases where regex
  # over text is too coarse — meta-variables and AST patterns instead.
  # The default nixpkgs build exposes `ast-grep` and disables the deprecated
  # `sg` compatibility binary to avoid colliding with shadow-utils.
  #
  # No `programs.ast-grep` HM module exists; direct home.packages install
  # follows the lazygit/gh pattern. Per-project rules live in
  # `sgconfig.yml` at the consumer repo root, so nothing user-global to
  # symlink here.
  home.packages = [ pkgs.ast-grep ];
}
