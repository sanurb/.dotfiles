{ pkgs, ... }:
{
  # awscli — the official AWS command-line interface. Pinned to the v2
  # package (`pkgs.awscli2`); v1 (`pkgs.awscli`) is EOL-track and carries
  # an older Python/botocore. v2 ships the `aws` binary plus the SSO and
  # `aws configure sso` flows that are the default auth path in 2026.
  #
  # Satellite (opt-out per host), not foundation: AWS access is a
  # work-specific need, and a host with no AWS footprint shouldn't carry
  # the closure. No `programs.awscli` HM module exists; direct
  # home.packages install follows the jo/procs/ast-grep pattern.
  #
  # Credentials and config (~/.aws/{config,credentials}) are host-local
  # state, deliberately outside this repo — a public-safe dotfiles tree
  # must never vendor access keys, role ARNs, or account IDs.
  home.packages = [ pkgs.awscli2 ];
}
