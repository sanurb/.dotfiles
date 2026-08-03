{
  config,
  lib,
  workspaceRoot,
  ...
}:
let
  piPackage = "@earendil-works/pi-coding-agent";
  vpHome = "${config.home.homeDirectory}/.vite-plus";
in
{
  # pi — terminal coding agent. This module owns pi end to end: the
  # global binary (below) and the live-edit config seams under
  # ~/.pi/agent/.
  #
  # NOTE ON THE INSTALLER: pi used to be installed with `vp install -g`
  # from modules/home/vite-plus.nix. That is now actively broken and must
  # not be reintroduced. vp stores global packages under
  #
  #   $VP_HOME/packages/@earendil-works/pi-coding-agent#<installId>/
  #
  # and that literal `#` is fatal to pi's own extension loader. pi
  # compiles TypeScript extensions through jiti, which builds the
  # importing module's URL by concatenating "file://" onto a raw path
  # instead of percent-encoding it (pathToFileURL). Node then parses
  # everything from `#` onward as a URL *fragment*, so bare-specifier
  # resolution restarts from the truncated directory
  # `$VP_HOME/packages/@earendil-works/pi-coding-agent`, which does not
  # exist. Every extension that does a runtime (value) import of
  # `@earendil-works/pi-coding-agent` then dies with
  # "Cannot find module '<truncated path>'"; type-only importers survive
  # only because their imports are erased before they ever reach Node.
  #
  # Nothing in vp lets us drop the `#<installId>` segment, so the fix is
  # to install pi from a path that has no `#` in it. bun's global prefix
  # (~/.bun/bin, already on home.sessionPath in foundation.nix alongside
  # the other out-of-Nix package managers) is exactly that.
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

  # Idempotent install hook, same shape as the vp hook in vite-plus.nix:
  # guard on the binary so a warm machine is a no-op, and fail soft so a
  # missing prerequisite prints a hint instead of aborting activation.
  #
  # `nh home switch` runs with a minimal PATH that lacks the proto shims,
  # so bun (proto-managed) has to be put back on PATH for the hook.
  home.activation.installPi = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    export PATH="$HOME/.proto/shims:$HOME/.proto/bin:$PATH"

    # Evict the old vp-managed shim if it is still around. Left in place
    # it wins PATH outright — $VP_HOME/bin is *prepended* by vp's
    # env.fish, ahead of everything foundation.nix sets — and every pi
    # extension with a runtime import silently fails to load. Only the
    # `pi` entries are removed; vp itself and its other shims stay.
    if [ -L "${vpHome}/bin/pi" ] || [ -e "${vpHome}/bin/pi" ]; then
      $VERBOSE_ECHO "pi: removing stale vp shim at ${vpHome}/bin/pi (breaks extension loading)"
      run rm -f "${vpHome}/bin/pi" "${vpHome}/bins/pi.json"
    fi

    if [ -x "$HOME/.bun/bin/pi" ]; then
      $VERBOSE_ECHO "pi: ${piPackage} already installed; skipping"
    elif command -v bun >/dev/null 2>&1; then
      $VERBOSE_ECHO "pi: installing ${piPackage} via bun"
      run bun add -g "${piPackage}"
    else
      echo "pi: bun not found on PATH — skipped 'bun add -g ${piPackage}'." >&2
      echo "  fix: install proto-pinned bun, then rerun \`dots apply\`." >&2
    fi
  '';
}
