{ pkgs, ... }:
{
  # jo — JSON output builder. The inverse of jq: composes JSON from shell
  # arguments (`jo name=ada age=36 admin=true`) instead of querying it.
  # Sibling to jaq in the JSON tier — jaq reads, jo writes — so ad-hoc
  # shell pipelines no longer need heredoc'd templates or printf-with-
  # quote-escaping to produce a payload for curl / gh api / kubectl.
  #
  # No `programs.jo` HM module exists; direct home.packages install
  # follows the procs/ast-grep pattern. There is no user config surface
  # — flags and nesting are positional, evaluated per invocation.
  home.packages = [ pkgs.jo ];
}
