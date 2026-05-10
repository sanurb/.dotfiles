{ pkgs, ... }:
{
  # jaq — jq-compatible JSON processor in Rust. Faster startup and a
  # stricter type system than jq, but accepts the same filter syntax for
  # the 90% case (see github.com/01mf02/jaq#differences-to-jq for the
  # corners that diverge — mostly around `path()` and `limit`).
  #
  # Installed alongside jq rather than replacing it: tooling that shells
  # out to `jq` by name (kubectl plugins, fish prompt helpers, gh
  # extensions) keeps working, and jaq is reached explicitly when its
  # speed matters.
  home.packages = [ pkgs.jaq ];
}
