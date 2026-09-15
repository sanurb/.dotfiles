{ ... }:
{
  # Granted — encrypted, SSO-aware AWS role selection. Run `assume` after
  # entering a Kubie shell so both AWS credentials and Kubernetes context die
  # with that shell instead of leaking into the parent session.
  #
  # Granted shares ~/.aws/config with awscli2 and does not replace `aws eks
  # get-token`, which EKS kubeconfigs commonly use as their exec credential
  # provider. Home Manager supplies the fish/zsh `assume` integration while
  # credentials and account identifiers remain host-local.
  programs.granted.enable = true;
}
