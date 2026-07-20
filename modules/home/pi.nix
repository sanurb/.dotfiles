{
  config,
  lib,
  workspaceRoot,
  ...
}:
{
  # pi — terminal coding agent. The binary itself is installed by
  # modules/home/vite-plus.nix (`vp install -g`); this module owns only
  # the live-edit config seams under ~/.pi/agent/.
  #
  # Unlike modules/home/opencode.nix we deliberately do NOT symlink the
  # whole ~/.pi tree: ~/.pi/agent/ is also pi's runtime home (auth.json,
  # sessions/, trust.json, settings.json, npm/), and a whole-tree link
  # would drag live credentials and session transcripts inside the repo
  # working tree. Only the customization surfaces below are repo-owned:
  #
  #   extensions/  TypeScript event hooks, tools, and slash commands
  #                (npm workspace — run `npm install` in config/pi once;
  #                node_modules is gitignored)
  #   skills/      user-level skills (vendored mattpocock/skills set +
  #                the Cloudflare docs bundle)
  #   themes/      TUI themes selectable from settings.json
  #   mcp.json     MCP server roster (proxy tool mode, lazy startup)
  #   cloak.json   secret-masking patterns for the pi-cloak extension
  #
  # force=true lets home-manager adopt the hand-made bootstrap symlinks
  # that pointed at the same targets before the first `dots apply`.
  home.file = lib.mkIf (workspaceRoot != "") (
    let
      seam = sub: {
        source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/pi/agent/${sub}";
        force = true;
      };
    in
    {
      ".pi/agent/extensions" = seam "extensions";
      ".pi/agent/skills" = seam "skills";
      ".pi/agent/themes" = seam "themes";
      ".pi/agent/mcp.json" = seam "mcp.json";
      ".pi/agent/cloak.json" = seam "cloak.json";
    }
  );
}
