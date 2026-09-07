{
  config,
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
let
  plannotatorTui = pkgs.callPackage ../packages/plannotator-tui.nix { };
  pluginSources = (builtins.fromTOML (builtins.readFile ../../config/herdr/plugins.toml)).plugins;
  syncHerdrPlugins = pkgs.writeShellApplication {
    name = "dots-herdr-plugins";
    runtimeInputs = [
      pkgs.git
      pkgs.curl
      pkgs.coreutils
      pkgs.bash
    ];
    text = ''
      export PATH="$HOME/.proto/shims:$HOME/.proto/bin:$HOME/.local/bin:${lib.makeBinPath [ pkgs.proto ]}:$PATH"
      export PLANNOTATOR_TUI_BIN="${plannotatorTui}/bin/plannotator-tui"
      set -- ${
        lib.escapeShellArgs (
          lib.concatMap (plugin: [
            plugin.source
            plugin.rev
          ]) pluginSources
        )
      }
      ${builtins.readFile ../scripts/herdr-plugins.sh}
    '';
  };
in
{
  home.packages = [
    plannotatorTui
    syncHerdrPlugins
  ]
  ++ lib.optionals pkgs.stdenv.hostPlatform.isLinux [
    pkgs.wl-clipboard
    pkgs.xclip
  ];

  # herdr — terminal workspace multiplexer with agent-aware panes
  # (herdr.dev). Not in nixpkgs. The binary is NOT installed here: it is
  # declared in config/agents/agents.toml and converged by
  # `dots agents sync`, which pins an exact version and verifies the
  # published SHA-256 before installing.
  #
  # It used to be installed by an activation hook that ran herdr.dev's
  # install.sh. That hook could not pin: install.sh hardcodes its
  # manifest URL to latest.json and reads the version out of it, so it
  # only ever lands "newest" — and the hook skipped entirely once the
  # binary existed, so a host never upgraded either. This module keeps
  # what it can actually own: configuration seams and plugins.
  #
  # Only config.toml is seamed. ~/.config/herdr/ doubles as herdr's
  # runtime home (herdr.sock, session.json, plugins/, logs, integration
  # locks) — a whole-dir link would drag live sockets and session state
  # into the repo working tree, the same hazard pi.nix documents for
  # ~/.pi/agent/. Integration shims herdr writes elsewhere
  # (config/opencode/plugins/herdr-agent-state.js and the pi equivalent)
  # are gitignored; regenerate them with `herdr integration`.
  #
  # force=true adopts the config.toml herdr's onboarding wrote before
  # this module owned it.
  xdg.configFile."herdr/config.toml" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/herdr/config.toml";
    force = true;
  };

  xdg.configFile."plannotator-tui/config.toml" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink "${workspaceRoot}/config/plannotator-tui/config.toml";
  };

  # Plugin installation needs the newly linked configuration and the herdr
  # binary. The plugin's supported binary override uses our verified Nix
  # package, so its build hook does not download another copy outside the
  # Nix closure.
  #
  # herdr now arrives from the sync-agents plan step, which `dots apply`
  # runs immediately before activation precisely so this hook finds the
  # binary on a fresh host. Activation can still legitimately run without
  # it — `dots apply --skip-agents`, or `nh home switch` invoked directly
  # — so the call stays fail-soft: the script detects the missing binary
  # and prints its own hint, and running it as an `if` condition exempts
  # it from activation's `set -e`. Same contract the install hooks
  # used to carry.
  home.activation.syncHerdrPlugins = lib.hm.dag.entryAfter [ "linkGeneration" ] ''
    if ! run ${syncHerdrPlugins}/bin/dots-herdr-plugins; then
      echo "herdr: plugin sync incomplete." >&2
      echo "  fix: run \`dots agents sync\`, then rerun \`dots apply\`." >&2
    fi
  '';
}
