{ ... }:
{
  # delta — git pager. enableGitIntegration owns core.pager and the
  # [delta] section in the rendered git config, so there's no parallel
  # block in modules/home/git.nix. syntax-theme matches bat
  # (modules/home/bat.nix).
  programs.delta = {
    enable = true;
    enableGitIntegration = true;
    options = {
      syntax-theme = "TwoDark";
      side-by-side = true;
      navigate = true;
      line-numbers = true;
    };
  };
}
