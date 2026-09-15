{ pkgs, ... }:
{
  # Standalone Kustomize — the independently released Kubernetes patching
  # engine. Prefer `kustomize build <overlay> | kubectl apply -f -` over
  # `kubectl apply -k` so manifest behavior follows the current Kustomize
  # release instead of the version embedded in kubectl.
  #
  # Kustomizations are project-local source, so there is no user-global
  # configuration for Home Manager to own.
  home.packages = [ pkgs.kustomize ];
}
