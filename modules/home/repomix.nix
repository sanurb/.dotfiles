{ pkgs, ... }:
{
  # repomix — packs a repository (or any subtree) into a single
  # token-counted, gitignore-aware artifact for feeding an LLM agent. Its
  # tree-sitter `--compress` mode keeps signatures and structure while
  # dropping function bodies, so a large module's *shape* costs a fraction
  # of its full text — the input-context complement to the output-side
  # trimming the rtk proxy already does on command results. Same goal
  # (spend fewer tokens to convey the same information), opposite end of
  # the pipe.
  #
  # Lives as a satellite (opt-out per host) rather than foundation: only
  # hosts that drive coding agents benefit, and a cluster/server persona
  # carries no value from it. No `programs.repomix` HM module exists;
  # direct home.packages install follows the ast-grep pattern. Output
  # shape is per-invocation or a project-local `repomix.config.json` at
  # the consumer repo root — nothing user-global to manage here.
  home.packages = [ pkgs.repomix ];
}
