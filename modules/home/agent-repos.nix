{
  config,
  lib,
  workspaceRoot,
  ...
}:
{
  # agent-repos — vendors third-party GitHub repos into a project as
  # squashed git subtrees under repos/, so coding agents read real
  # upstream source instead of hallucinating APIs. Ported verbatim from
  # dmmulroy/.dotfiles (home/.local/bin/agent-repos); no personal hosts.
  #
  # Needs bash 4+ (arrays, BASH_REMATCH) — /usr/bin/env bash resolves to
  # the HM-managed bash, not Apple's 3.2 — plus git-subtree and,
  # optionally, python3 for repo-URL inference. The fish completion shim
  # ships with the fish satellite (config/fish/completions/agent-repos.fish).
  #
  # First repo-managed ~/.local/bin entry: ~/.local/bin itself stays an
  # unmanaged escape hatch (claude, codex, herdr, uv install there);
  # only this one file is a seam into config/bin/.
  home.file = lib.mkIf (workspaceRoot != "") {
    ".local/bin/agent-repos" = {
      source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/bin/agent-repos";
      force = true;
    };
  };
}
