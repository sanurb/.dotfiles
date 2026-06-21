{ pkgs, ... }:
{
  # yq (mikefarah, Go) — a jq-compatible processor for YAML / TOML / XML /
  # properties / CSV, the structured-text surface this repo actually swims
  # in (devenv, moon, GitHub Actions, k8s manifests, .prototools). Sibling
  # to jaq (modules/home/jaq.nix): jaq owns JSON, yq owns everything else,
  # and together with htmlq (HTML) they give agents and pipelines a
  # uniform jq grammar over every structured format without a bespoke
  # parser per file type.
  #
  # Deliberately `pkgs.yq-go`, NOT `pkgs.yq`: nixpkgs ships two unrelated
  # tools that both install a `yq` binary — the Python `yq` (a thin jq
  # wrapper by kislyuk) and this Go reimplementation by mikefarah. The Go
  # one is standalone (no jq/Python runtime), faster, and preserves YAML
  # comments and key order on round-trips, which the wrapper cannot. The
  # installed binary is still named `yq`.
  #
  # No `programs.yq` HM module exists; direct install follows the
  # jaq/ast-grep pattern. No user-global config to manage.
  home.packages = [ pkgs.yq-go ];
}
