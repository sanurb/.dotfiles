{
  description = "dots";

  # Public binary caches advertised to anyone who evaluates this flake.
  # `dots apply` already runs nh with `--accept-flake-config` (see
  # apps/cli/cmd_apply.go's activationArgs), so the substituters and
  # keys below are picked up automatically without prompting. A user
  # invoking the flake directly (`nix develop`, `nix build .#...`)
  # will see the standard "do you want to allow these?" prompt unless
  # they have `accept-flake-config = true` in nix.conf.
  #
  #   nix-community: hosts the bulk of nix-community packages
  #     (home-manager activations, neovim wrappers, etc.) — major
  #     hit-rate win on first activation of a fresh host.
  #   colmena: deployment tooling cache; pulled in transitively via
  #     some home-manager modules.
  nixConfig = {
    extra-substituters = [
      "https://nix-community.cachix.org"
      "https://colmena.cachix.org"
    ];
    extra-trusted-public-keys = [
      "nix-community.cachix.org-1:mB9FSh9qf2dCimDSUo8Zy7bkq5CX+/rkCWyvRCYg3Fs="
      "colmena.cachix.org-1:7BzpDnjjH8ki2CT3f6GdOk7QAzPOl+1t3LvTLXqYcSg="
    ];
  };

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    # Per-tool aggressive pin. Modules whose upstream cycles faster than
    # `nixos-unstable`'s hydra gating opt into this via `pkgsPins.edge`.
    # See docs/maintenance.md for the divergence log; this pin collapses
    # back to `nixpkgs` when no module depends on it for ≥ 90 days.
    nixpkgs-edge.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-parts.inputs.nixpkgs-lib.follows = "nixpkgs";

    devenv.url = "github:cachix/devenv";

    # Workspace-root sentinel for devenv's eval-time `devenv.root` option.
    # devenv.lib.mkShell asserts a known project directory; in pure eval,
    # `./.` resolves to the immutable nix store path (write fails at build
    # time), so we need an *external* file containing the absolute workspace
    # path. The .envrc rewrites .devenv-root with $PWD before invoking
    # `nix develop --override-input devenv-root file+file://$PWD/.devenv-root`,
    # capturing the right value for the active checkout. Without the
    # override (e.g., a fresh `nix flake check` in CI), this defaults to
    # /dev/null → empty content → the devShell falls back to a no-op shell
    # so eval/build still succeeds. This is the canonical pattern from
    # https://devenv.sh/guides/using-with-flakes/.
    devenv-root = {
      url = "file+file:///dev/null";
      flake = false;
    };

    treefmt-nix.url = "github:numtide/treefmt-nix";
    treefmt-nix.inputs.nixpkgs.follows = "nixpkgs";

    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = inputs@{ self, nixpkgs, nixpkgs-edge, flake-parts, devenv, home-manager, ... }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];

      # Single source of truth for the per-system Home Manager configuration.
      # Used by `homeConfigurations` (for `home-manager switch --flake`),
      # `packages.homeActivation` (the activation derivation `dots apply`
      # invokes via nh), and `checks.home-activation` (the `nix flake check`
      # gate that fails a PR when the activation derivation can't build).
      # Factor here so extraSpecialArgs is declared once.
      mkHome = system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in home-manager.lib.homeManagerConfiguration {
          inherit pkgs;
          modules = [ ./modules/profiles/home.nix ];
          extraSpecialArgs = {
            inherit inputs system;
            # Modules pull from `pkgsPins.<name>` when they need a different
            # nixpkgs hash than the default. Adding a key here is the only
            # supported way to introduce a per-tool pin — modules don't reach
            # into inputs directly. Keeps the pin set discoverable in one
            # place and keeps the maintenance log honest.
            pkgsPins = {
              edge = nixpkgs-edge.legacyPackages.${system};
            };
          };
        };
    in
    flake-parts.lib.mkFlake { inherit inputs; } {
      inherit systems;

      perSystem = { system, ... }:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          # Read the workspace path from the `devenv-root` flake input.
          # Empty content (the /dev/null default) means: no override was
          # supplied, so fall back to a no-op shell that still lets the
          # gate pass without devenv eval/build. The .envrc supplies the
          # override for interactive entry.
          devenvRootContent = builtins.readFile inputs.devenv-root.outPath;
          devenvRoot = nixpkgs.lib.removeSuffix "\n" devenvRootContent;
          haveDevenvRoot = devenvRoot != "";
        in
        {
          devShells.default =
            if haveDevenvRoot then
              devenv.lib.mkShell
                {
                  inherit inputs pkgs;
                  modules = [
                    { devenv.root = devenvRoot; }
                    ./devenv.nix
                  ];
                }
            else
            # Fallback so `nix flake check` from a non-direnv shell
            # still passes. `nix develop` against this output prints
            # actionable guidance and exits non-zero rather than
            # silently dropping the user into a barebones shell.
              pkgs.mkShellNoCC {
                name = "dots-shell-no-devenv-root";
                shellHook = ''
                  cat <<'MSG' >&2
                  ERROR: devenv-root override missing.

                  The dots dev shell needs an absolute path to the workspace
                  so devenv can write its state under \$DEVENV_ROOT/.devenv.
                  Enter the workspace via 'direnv allow' (the .envrc supplies
                  the override automatically), or invoke nix develop manually:

                    nix develop --impure --accept-flake-config \
                      --override-input devenv-root \
                      "file+file://$(pwd)/.devenv-root" \
                      -c <command>

                  ('.devenv-root' must contain the absolute workspace path; the
                  .envrc maintains this file for you when direnv is enabled.)
                  MSG
                  exit 1
                '';
              };

          packages = {
            homeActivation = (mkHome system).activationPackage;
            home-manager = home-manager.packages.${system}.default;
          };

          # `nix flake check` runs every entry here. Keep them cheap — the
          # gate is only useful if it fits in the verification budget.
          checks = {
            # Schema parity is structural: Go embeds the SCHEMA_VERSION file,
            # Nix reads the same file. This check just guards the file's
            # well-formedness so a bad commit fails fast rather than panicking
            # the Go binary at startup or throwing during Nix eval.
            schema-version-wellformed = pkgs.runCommand "schema-version-wellformed" { } ''
              ver=$(${pkgs.coreutils}/bin/cat ${./apps/cli/internal/state/SCHEMA_VERSION} | ${pkgs.coreutils}/bin/tr -d '[:space:]')
              case "$ver" in
                ""|*[!0-9]*)
                  echo "SCHEMA_VERSION must be a positive integer, got: '$ver'" >&2
                  exit 1
                  ;;
              esac
              if [ "$ver" -lt 1 ]; then
                echo "SCHEMA_VERSION must be >= 1, got: $ver" >&2
                exit 1
              fi
              echo "ok schema_version=$ver" > $out
            '';

            # Building the activation derivation in CI is the gate that turns
            # "did we ship a broken activation?" from a release-time
            # discovery into a PR-time failure. Catches upstream
            # regressions (e.g., a neovim wrapper bug that breaks
            # rplugin.vim) and home-manager option-rename breaks before
            # they reach a user's `dots apply`.
            home-activation = (mkHome system).activationPackage;

            # Persona-shell ↔ source-tree linkage. The wizard's "fish"
            # selection has to land both as a binary (programs.fish.enable)
            # and as a config-tree symlink (xdg.configFile."fish") — the
            # latter is what makes the imported config/fish/ tree
            # actually reach ~/.config/fish/ at activation time. A
            # missing xdg.configFile."fish" entry is exactly the bug the
            # user reported as "fish was selected but did not end up
            # installed": the binary lands but their abbreviations,
            # functions, and conf.d entries do not.
            #
            # We assert the wiring by inspecting the resolved HM config
            # (no rebuild — just a property of the eval result) for the
            # presence of the "fish" key under config.xdg.configFile.
            fish-config-wired =
              let
                # Build an HM config with a non-empty workspaceRoot so
                # the lib.mkIf gate inside fish.nix actually evaluates
                # its body. `nix flake check` runs in pure mode where
                # builtins.getEnv "DOTS_WORKSPACE_ROOT" returns "" —
                # without the override, the mkIf would collapse and
                # the check would always pass whether or not fish.nix
                # declares the entry.
                evaluated = (home-manager.lib.homeManagerConfiguration {
                  inherit pkgs;
                  modules = [
                    ./modules/profiles/home.nix
                    { _module.args.workspaceRoot = pkgs.lib.mkForce "/synthetic/dots"; }
                  ];
                  extraSpecialArgs = {
                    inherit inputs system;
                    pkgsPins.edge = nixpkgs-edge.legacyPackages.${system};
                  };
                }).config;
                # The fish module symlinks per-subtree rather than the
                # whole fish directory (HM owns config.fish), so we
                # assert that at least conf.d and functions are wired.
                # Either missing means the user's curated tree won't
                # reach ~/.config/fish/.
                ok = evaluated.xdg.configFile ? "fish/conf.d"
                  && evaluated.xdg.configFile ? "fish/functions";
              in
              pkgs.runCommand "fish-config-wired" { } (
                if ok then ''
                  echo "ok: modules/home/shells/fish.nix wires fish/conf.d + fish/functions" > $out
                '' else ''
                  echo "modules/home/shells/fish.nix is missing xdg.configFile entries for fish subtrees" >&2
                  echo "  why: without them, config/fish/ is never linked into ~/.config/fish/" >&2
                  echo "  fix: declare xdg.configFile.\"fish/conf.d\" and \"fish/functions\" via mkOutOfStoreSymlink" >&2
                  exit 1
                ''
              );
          };

          formatter = pkgs.nixpkgs-fmt;
        };

      # Each value must BE a homeConfiguration (carrying `.activationPackage`
      # directly) — `nix flake check` walks this attrset and inspects each
      # entry's `.activationPackage.system`. Keying by system gives:
      #   home-manager switch --flake .#aarch64-darwin
      # while Moon's deploy uses packages.${system}.homeActivation.
      flake.homeConfigurations = nixpkgs.lib.genAttrs systems mkHome;
    };
}
