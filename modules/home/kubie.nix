{ pkgs, ... }:
{
  # Kubie — context and namespace selection through isolated shells. `kubie
  # ctx` and `kubie exec` project one context into a temporary kubeconfig and
  # set KUBECONFIG/KUBIE_ACTIVE only for the child environment, leaving source
  # kubeconfigs and sibling terminals untouched.
  #
  # Source kubeconfigs remain host-local under ~/.kube; this module deliberately
  # manages no cluster endpoints or credentials. Kubie's upstream is seeking
  # additional maintainers (kubie-org/kubie#385), so keep it an independently
  # removable satellite rather than coupling other Kubernetes tools to it.
  home.packages = [ pkgs.kubie ];
}
