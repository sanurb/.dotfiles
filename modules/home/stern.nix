{ pkgs, ... }:
{
  # Stern — concurrent, color-coded log streaming across matching Kubernetes
  # pods and containers. It honors KUBECONFIG, so invoking it inside `kubie
  # ctx` or through `kubie exec` keeps log access pinned to the isolated
  # context and namespace.
  #
  # Defaults remain invocation-local; no global Stern config is managed.
  home.packages = [ pkgs.stern ];
}
