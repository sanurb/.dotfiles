{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
let
  piConfigRoot = "${workspaceRoot}/config/pi";
  piWebToolsRoot = "${piConfigRoot}/agent/extensions/web-tools";
  vpHome = "${config.home.homeDirectory}/.vite-plus";

  # PATH prefix for the extension-dependency hook below. The shim dirs
  # lead so a workspace
  # .prototools pin still wins, but proto's own bin dir has to be on PATH
  # too: proto is installed from nixpkgs (foundation.nix), so the binary
  # lives in the Home Manager profile, NOT at ~/.proto/bin/proto where
  # proto-shim looks for it. Under `nh home switch`'s minimal PATH the
  # shims therefore cannot exec proto and every shimmed command dies with
  #   proto-shim: Failed to execute proto for the shimmed command:
  #   No such file or directory (os error 2)
  # which used to abort activation. Pinning the store path keeps the hook
  # independent of whatever the caller's PATH happens to be.
  #
  # `dots agents sync` needs none of this: it runs from an interactive
  # shell whose PATH already carries the proto shims, which is one of the
  # reasons agent installs no longer belong in activation.
  protoPath = "$HOME/.proto/shims:$HOME/.proto/bin:${lib.makeBinPath [ pkgs.proto ]}";
in
{
  # pi — terminal coding agent. This module owns pi's live-edit config
  # seams under ~/.pi/agent/. It does NOT own the binary: pi is declared
  # in config/agents/agents.toml and converged by `dots agents sync`,
  # which installs an exact `pkg@version` instead of whatever `latest`
  # resolved to on the day a host was first provisioned.
  #
  # The install hook this replaced guarded on `[ -x ~/.bun/bin/pi ]`, so
  # it ran exactly once per host and never upgraded. Two machines
  # applying the same commit ended up on different builds of pi, with the
  # version recorded nowhere in the repo.
  #
  # NOTE ON THE INSTALLER: the BOM installs pi through bun's global
  # prefix, not vp or npm, and that choice is load-bearing. pi used to be
  # installed with `vp install -g` from modules/home/vite-plus.nix. That
  # is now actively broken and must not be reintroduced. vp stores global
  # packages under
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
  # the other out-of-Nix package managers) is exactly that — hence
  # `provider = "bun"` on the pi row in the BOM.
  #
  # Unlike modules/home/opencode.nix we deliberately do NOT symlink the
  # whole ~/.pi tree: ~/.pi/agent/ is also pi's runtime home (auth.json,
  # sessions/, trust.json, settings.json, npm/), and a whole-tree link
  # would drag live credentials and session transcripts inside the repo
  # working tree. Only the customization surfaces below are repo-owned:
  #
  #   extensions/  TypeScript event hooks, tools, and slash commands
  #                (npm workspace; runtime dependencies are installed by
  #                installPiExtensionDependencies below; node_modules is
  #                gitignored)
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

  # Auto-discovered local extensions are not package installs: Pi loads
  # their TypeScript directly and never runs npm install for package.json.
  # web-tools therefore needs its declared runtime dependencies installed
  # beside the real extension path under config/pi, where Node's upward
  # module resolution can find them after ~/.pi/agent/extensions resolves
  # through the Home Manager symlink.
  #
  # Resolve every declared dependency dynamically instead of duplicating
  # the package list in Nix. A warm machine is a no-op; a fresh clone runs
  # a workspace-scoped install that leaves unrelated extension workspaces
  # and the workspace root out of scope.
  home.activation.installPiExtensionDependencies = lib.mkIf (workspaceRoot != "") (
    # writeBoundary only: this resolves node modules under config/pi and
    # never invokes pi, so it has no ordering relationship with the
    # binary (which `dots agents sync` owns anyway).
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      export PATH="${protoPath}:$PATH"

      if (
        node --input-type=commonjs - ${lib.escapeShellArg piWebToolsRoot} >/dev/null 2>&1 <<'NODE'
      const { createRequire } = require("node:module");
      const { readFileSync } = require("node:fs");
      const { join } = require("node:path");

      const extensionRoot = process.argv[2];
      const manifest = JSON.parse(readFileSync(join(extensionRoot, "package.json"), "utf8"));
      const requireFromExtension = createRequire(join(extensionRoot, "index.ts"));

      for (const dependency of Object.keys(manifest.dependencies ?? {})) {
        requireFromExtension.resolve(dependency);
      }
      NODE
      ); then
        $VERBOSE_ECHO "pi: web-tools runtime dependencies already installed; skipping"
      elif command -v npm >/dev/null 2>&1; then
        $VERBOSE_ECHO "pi: installing web-tools runtime dependencies"
        if (
          cd ${lib.escapeShellArg piConfigRoot}
          run npm install --ignore-scripts --workspace=pi-web-tools-extension --include-workspace-root=false
        ); then
          $VERBOSE_ECHO "pi: web-tools runtime dependencies installed"
        else
          echo "pi: failed to install web-tools runtime dependencies." >&2
          echo "  fix: run \`cd ${piConfigRoot} && npm install\`, then rerun \`dots apply\`." >&2
        fi
      else
        echo "pi: npm not found on PATH — skipped web-tools runtime dependencies." >&2
        echo "  fix: install the proto-pinned Node/npm runtimes, then rerun \`dots apply\`." >&2
      fi
    ''
  );

  # Evict the old vp-managed pi shim. This is a PATH repair, not an
  # install, which is why it outlived the install hook it used to share
  # a block with: left in place the shim wins PATH outright —
  # $VP_HOME/bin is *prepended* by vp's env.fish, ahead of everything
  # foundation.nix sets — and every pi extension with a runtime import
  # silently fails to load. Only the `pi` entries are removed; vp itself
  # and its other shims stay.
  home.activation.evictStaleVpPiShim = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    if [ -L "${vpHome}/bin/pi" ] || [ -e "${vpHome}/bin/pi" ]; then
      $VERBOSE_ECHO "pi: removing stale vp shim at ${vpHome}/bin/pi (breaks extension loading)"
      run rm -f "${vpHome}/bin/pi" "${vpHome}/bins/pi.json"
    fi
  '';
}
