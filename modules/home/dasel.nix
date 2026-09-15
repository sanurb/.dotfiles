{ pkgs, ... }:
{
  # dasel (Data-Select) — queries, edits, and converts structured data with
  # one selector language across JSON, YAML, TOML, XML, CSV, HCL, INI, and
  # KDL. It owns multi-format selection, mutation, and conversion; jaq remains
  # the focused jq-compatible fast path for JSON-only pipelines.
  #
  # No `programs.dasel` Home Manager module exists; direct home.packages
  # installation follows the jaq/ast-grep pattern. Dasel has no user-global
  # configuration to manage.
  home.packages = [ pkgs.dasel ];
}
