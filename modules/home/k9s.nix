{ pkgs, ... }:
{
  # k9s — real-time terminal UI for navigating, observing, and managing
  # Kubernetes clusters. The interactive layer over the kubectl satellite
  # (modules/home/kubectl.nix): both read the same host-local kubeconfig
  # (~/.kube/config), so a host that opts into one almost always wants the
  # other — k9s for live exploration, kubectl for scripted/precise ops.
  #
  # Direct home.packages install, NOT `programs.k9s`. The HM module does
  # exist, but with no skin / hotkey / plugin to vendor it would only
  # materialize an empty managed ~/.config/k9s tree, and k9s config is
  # host-local taste — the same judgement procs.nix and kubectl.nix make
  # for their optional config. Leaving it unmanaged lets a host drop in a
  # skin or plugin without a flake rebuild.
  #
  # Cluster context (endpoints, credentials) stays host-local and
  # deliberately outside this public-safe repo — a dotfiles tree must
  # never vendor cluster access.
  home.packages = [ pkgs.k9s ];
}
