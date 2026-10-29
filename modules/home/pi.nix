{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
let
  # The imported codemode/provider APIs require the Pi 1.4 release line.
  piPackage = "@earendil-works/pi-coding-agent@^1.4.0";
  piConfigRoot = "${workspaceRoot}/config/pi";
  cuaDriverVersion = "0.24.0";
  cuaDriverInstallDir = "${config.home.homeDirectory}/.local/bin";
  cuaDriverInstallerPath =
    lib.makeBinPath [
      pkgs.bash
      pkgs.coreutils
      pkgs.curl
      pkgs.gawk
      pkgs.gzip
      pkgs.gnugrep
      pkgs.gnused
      pkgs.gnutar
    ]
    + lib.optionalString pkgs.stdenv.hostPlatform.isDarwin ":/usr/bin:/usr/sbin";
  vpHome = "${config.home.homeDirectory}/.vite-plus";

  # PATH prefix for the hooks below. The shim dirs lead so a workspace
  # .prototools pin still wins, but proto's own bin dir has to be on PATH
  # too: proto is installed from nixpkgs (foundation.nix), so the binary
  # lives in the Home Manager profile, NOT at ~/.proto/bin/proto where
  # proto-shim looks for it. Under `nh home switch`'s minimal PATH the
  # shims therefore cannot exec proto and every shimmed command dies with
  #   proto-shim: Failed to execute proto for the shimmed command:
  #   No such file or directory (os error 2)
  # which used to abort activation on `installPi`. Pinning the store path
  # keeps the hook independent of whatever the caller's PATH happens to be.
  protoPath = "$HOME/.proto/shims:$HOME/.proto/bin:${lib.makeBinPath [ pkgs.proto ]}";
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
  #                (npm workspace; runtime dependencies are installed by
  #                installPiExtensionDependencies below; node_modules is
  #                gitignored)
  #   skills/      user-level skills (vendored mattpocock/skills set +
  #                the Cloudflare docs bundle)
  #   themes/      TUI themes selectable from settings.json
  #   mcp.json     Native Pi MCP server roster (codemode exposure)
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
  # Resolve runtime dependencies from the root and every extension manifest:
  # Herdr's SDK needs Effect/Ajv just as web-tools needs its HTML adapters.
  # Keep the package list in package.json rather than a second Nix roster.
  # Resolve with import conditions: Pi libraries expose ESM-only entry points.
  # A warm machine is a no-op; a fresh clone installs the shared workspace.
  home.activation.installPiExtensionDependencies = lib.mkIf (workspaceRoot != "") (
    lib.hm.dag.entryAfter
      [
        "writeBoundary"
        "installPi"
      ]
      ''
        export PATH="${protoPath}:$PATH"

        if (
          node --conditions=import --input-type=commonjs - ${lib.escapeShellArg piConfigRoot} >/dev/null 2>&1 <<'NODE'
        const { createRequire } = require("node:module");
        const { existsSync, readFileSync, readdirSync } = require("node:fs");
        const { join } = require("node:path");

        const configRoot = process.argv[2];
        const extensionsRoot = join(configRoot, "agent", "extensions");
        const packageRoots = [configRoot, ...readdirSync(extensionsRoot, { withFileTypes: true })
          .filter(entry => entry.isDirectory())
          .map(entry => join(extensionsRoot, entry.name))];

        for (const packageRoot of packageRoots) {
          const manifestPath = join(packageRoot, "package.json");
          if (!existsSync(manifestPath)) continue;
          const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
          const requireFromExtension = createRequire(manifestPath);
          for (const dependency of Object.keys(manifest.dependencies ?? {})) {
            requireFromExtension.resolve(dependency);
          }
        }
        NODE
        ); then
          $VERBOSE_ECHO "pi: extension runtime dependencies already installed; skipping"
        elif command -v npm >/dev/null 2>&1; then
          $VERBOSE_ECHO "pi: installing extension runtime dependencies"
          if (
            cd ${lib.escapeShellArg piConfigRoot}
            run npm install --ignore-scripts --workspaces --include-workspace-root
          ); then
            $VERBOSE_ECHO "pi: extension runtime dependencies installed"
          else
            echo "pi: failed to install extension runtime dependencies." >&2
            echo "  fix: run \`cd ${piConfigRoot} && npm install\`, then rerun \`dots apply\`." >&2
          fi
        else
          echo "pi: npm not found on PATH — skipped extension runtime dependencies." >&2
          echo "  fix: install the proto-pinned Node/npm runtimes, then rerun \`dots apply\`." >&2
        fi
      ''
  );

  # Cua Driver backs the persistent `computer` MCP server declared in
  # config/pi/agent/mcp.json. Its macOS release must be installed through
  # the upstream installer rather than copied into the Nix store: the signed
  # /Applications/CuaDriver.app bundle is the stable identity that owns the
  # Accessibility and Screen Recording grants. Linux uses the same installer
  # and keeps its versioned payload under ~/.cua-driver.
  #
  # Pin the release that matches the vendored skill. The installer verifies
  # the release checksum and avoids editing shell startup files. As with
  # installPi below, a network or permission failure reports an actionable
  # recovery without aborting the rest of Home Manager activation.
  home.activation.installCuaDriver = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    cua_driver_binary="$(command -v cua-driver 2>/dev/null || true)"
    if [ -z "$cua_driver_binary" ] && [ -x "${cuaDriverInstallDir}/cua-driver" ]; then
      cua_driver_binary="${cuaDriverInstallDir}/cua-driver"
    fi
    cua_driver_reported_version=""
    if [ -n "$cua_driver_binary" ]; then
      cua_driver_reported_version="$("$cua_driver_binary" --version 2>/dev/null || true)"
    fi

    case "$cua_driver_reported_version" in
      *'${cuaDriverVersion}'*)
        $VERBOSE_ECHO "pi: cua-driver ${cuaDriverVersion} already installed; skipping installer"
        ;;
      *)
        installer="$(${pkgs.coreutils}/bin/mktemp)"
        $VERBOSE_ECHO "pi: installing cua-driver ${cuaDriverVersion}"
        if run ${pkgs.curl}/bin/curl \
          --proto '=https' \
          --tlsv1.2 \
          --fail \
          --silent \
          --show-error \
          --location \
          --retry 3 \
          --connect-timeout 10 \
          --max-time 30 \
          https://cua.ai/driver/install.sh \
          --output "$installer" \
          && run ${pkgs.coreutils}/bin/env \
            PATH="${cuaDriverInstallerPath}:$PATH" \
            CUA_DRIVER_RS_VERSION="${cuaDriverVersion}" \
            CUA_DRIVER_RS_INSTALL_DIR="${cuaDriverInstallDir}" \
            CUA_DRIVER_RS_NO_MODIFY_PATH=1 \
            CUA_DRIVER_RS_TELEMETRY_ENABLED=0 \
            ${pkgs.bash}/bin/bash "$installer" --no-modify-path
        then
          $VERBOSE_ECHO "pi: cua-driver installed"
        fi
        ${pkgs.coreutils}/bin/rm -f "$installer"
        ;;
    esac

    if [ ! -x "${cuaDriverInstallDir}/cua-driver" ] && ! command -v cua-driver >/dev/null 2>&1; then
      echo "pi: cua-driver was NOT installed (installer failed)." >&2
      echo '  fix: run `CUA_DRIVER_RS_VERSION=${cuaDriverVersion} /bin/bash -c "$(curl -fsSL https://cua.ai/driver/install.sh)"`, then rerun `dots apply`.' >&2
    fi
  '';

  # Idempotent install hook, same shape as the vp hook in vite-plus.nix:
  # guard on a compatible binary so a warm machine is a no-op, and fail soft so a
  # missing prerequisite prints a hint instead of aborting activation.
  #
  # `nh home switch` runs with a minimal PATH that lacks bun (proto-managed)
  # and proto itself, so protoPath puts both back for the hook.
  #
  # `bun add -g` runs as an `if` condition rather than a bare statement:
  # activation scripts use `set -e`, from which condition commands are
  # exempt. Without that, a bun failure aborts the whole activation
  # instead of printing the fix hint — which is what the fail-soft
  # contract above promises. Same reasoning as vite-plus.nix.
  home.activation.installPi = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    export PATH="${protoPath}:$PATH"

    # Evict the old vp-managed shim if it is still around. Left in place
    # it wins PATH outright — $VP_HOME/bin is *prepended* by vp's
    # env.fish, ahead of everything foundation.nix sets — and every pi
    # extension with a runtime import silently fails to load. Only the
    # `pi` entries are removed; vp itself and its other shims stay.
    if [ -L "${vpHome}/bin/pi" ] || [ -e "${vpHome}/bin/pi" ]; then
      $VERBOSE_ECHO "pi: removing stale vp shim at ${vpHome}/bin/pi (breaks extension loading)"
      run rm -f "${vpHome}/bin/pi" "${vpHome}/bins/pi.json"
    fi

    pi_version=""
    if [ -x "$HOME/.bun/bin/pi" ]; then
      pi_version="$("$HOME/.bun/bin/pi" --version 2>/dev/null || true)"
    fi
    if node -e '
      const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(process.argv[1]);
      process.exit(match && Number(match[1]) === 1 && Number(match[2]) >= 4 ? 0 : 1);
    ' "$pi_version"; then
      $VERBOSE_ECHO "pi: compatible version $pi_version already installed; skipping"
    elif command -v bun >/dev/null 2>&1; then
      $VERBOSE_ECHO "pi: installing ${piPackage} via bun"
      if run bun add -g "${piPackage}"; then
        $VERBOSE_ECHO "pi: ${piPackage} installed"
      else
        echo "pi: \`bun add -g ${piPackage}\` failed." >&2
        echo "  fix: run it manually, then rerun \`dots apply\`." >&2
      fi
    else
      echo "pi: bun not found on PATH — skipped 'bun add -g ${piPackage}'." >&2
      echo "  fix: install proto-pinned bun, then rerun \`dots apply\`." >&2
    fi
  '';
}
