{ pkgs, ... }:
{
  # htmlq — "jq for HTML": extracts content from an HTML document on
  # stdin via CSS selectors (`htmlq 'a.title' --text`, `--attribute href`,
  # …). It owns HTML extraction alongside jaq's JSON querying and dasel's
  # multi-format editing: an agent fetching a page with xh (foundation) can
  # slice the parts it needs through a CSS selector instead of paying for the
  # whole DOM as tokens — no headless browser, no regex-over-markup fragility.
  #
  # No `programs.htmlq` HM module exists; direct home.packages install
  # follows the jaq/ast-grep pattern. Nothing user-global to manage.
  home.packages = [ pkgs.htmlq ];
}
