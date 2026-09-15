{ pkgs, ... }:
{
  # awscli — the official AWS command-line interface. Pinned to the v2
  # package (`pkgs.awscli2`); v1 (`pkgs.awscli`) is EOL-track and carries
  # an older Python/botocore. v2 ships the `aws` binary plus the SSO and
  # `aws configure sso` flows that are the default auth path in 2026.
  #
  # Satellite (opt-out per host), not foundation: AWS access is a
  # work-specific need, and a host with no AWS footprint shouldn't carry
  # the closure. Granted handles role selection, but awscli2 remains required
  # by EKS kubeconfigs that execute `aws eks get-token` for authentication.
  #
  # This module installs only the binary: it never runs `aws eks
  # update-kubeconfig` or mutates ~/.kube/config. AWS config, credentials,
  # role ARNs, and account IDs remain host-local and outside this public repo.
  home.packages = [ pkgs.awscli2 ];
}
