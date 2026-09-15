{ ... }:
{
  # difftastic (`difft`) compares source by syntax rather than lines, so
  # formatting-only edits do not bury the structural change. Unsupported
  # languages and parse errors fall back to a line-oriented diff. YAML support
  # makes it useful for inspecting structural manifest changes after rendering
  # a Kustomize overlay.
  #
  # Do not replace `git diff` globally: difftastic output is for inspection
  # and cannot produce patches. Opt-in aliases preserve standard Git behavior
  # while exposing the upstream-recommended external-diff integration.
  programs.difftastic.enable = true;
  programs.git.settings.alias = {
    dft = "-c diff.external=difft diff";
    ds = "-c diff.external=difft show --ext-diff";
    dl = "-c diff.external=difft log -p --ext-diff";
  };
}
