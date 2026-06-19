{ pkgs, ... }:
{
  # kubectl — the Kubernetes CLI. Talks to any cluster the active
  # kubeconfig points at; no daemon, no cluster-side install. Lives as a
  # satellite (opt-out per host) rather than foundation because not every
  # persona touches Kubernetes — a laptop with no cluster access carries
  # no benefit from the binary.
  #
  # No `programs.kubectl` HM module exists; direct home.packages install
  # follows the jo/procs/ast-grep pattern. Cluster context is host-local
  # state (~/.kube/config), deliberately outside this repo — a public-safe
  # dotfiles tree must never vendor cluster credentials or endpoints.
  # Completion is left to the shells' generic machinery (zsh's
  # enableCompletion; fish loads `kubectl completion fish` on demand) —
  # no per-tool wiring to declare here.
  home.packages = [ pkgs.kubectl ];
}
