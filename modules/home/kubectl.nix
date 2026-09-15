{ pkgs, ... }:
{
  # kubectl — the Kubernetes CLI. Talks to any cluster the active
  # kubeconfig points at; no daemon, no cluster-side install. Lives as a
  # satellite (opt-out per host) rather than foundation because not every
  # persona touches Kubernetes — a laptop with no cluster access carries
  # no benefit from the binary.
  #
  # No `programs.kubectl` HM module exists; direct home.packages installation
  # follows the procs/ast-grep pattern. Enter through `kubie ctx` or
  # `kubie exec` for normal operation: Kubie supplies a temporary KUBECONFIG
  # scoped to that child environment, preventing one terminal from changing
  # another terminal's active context.
  #
  # Cluster credentials and endpoints remain host-local under ~/.kube. For
  # manifest rendering, prefer standalone `kustomize build ... | kubectl
  # apply -f -` over the version-lagged `kubectl apply -k` engine.
  home.packages = [ pkgs.kubectl ];
}
