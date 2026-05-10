{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
{
  # opencode — terminal-native AI coding agent. Installed straight from
  # nixpkgs; no `programs.opencode` HM module exists upstream, so this
  # module just lands the binary plus the live-edit config tree.
  #
  # API keys live in the user's environment (opencode reads from
  # ANTHROPIC_API_KEY / OPENAI_API_KEY / etc. directly), not in the
  # config file — the deny-list in opencode.json's permission.bash
  # already blocks `printenv`/`env`/`export` so the agent can't read
  # them back out at runtime.
  home.packages = [ pkgs.opencode ];

  # Live-editable seam — same pattern as modules/home/editor.nix and
  # modules/home/multiplexers/tmux.nix. The whole config/opencode/
  # tree (settings JSON, AGENTS.md, agents, commands, skills, plugins,
  # tools) becomes ~/.config/opencode/ via a single out-of-store
  # symlink. Edits to any prompt or skill markdown reach the running
  # tool on its next session start with no Nix evaluation; opencode's
  # plugin/tool TS files are picked up the same way.
  #
  # When workspaceRoot is empty (HM run outside `dots apply`) we skip
  # the symlink rather than emit a dangling pointer to "/" — opencode
  # falls back to its built-in defaults instead.
  xdg.configFile."opencode" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/opencode";
  };
}
