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

    # devenv's treefmt module reads this input directly; dropping it
    # produces a "To use 'treefmt', add the following to flake.nix"
    # error during enterShell even though the rest of the flake still
    # evaluates. devenv uses it, we keep it.
    treefmt-nix.url = "github:numtide/treefmt-nix";
    treefmt-nix.inputs.nixpkgs.follows = "nixpkgs";

    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";

    # Pre-built nix-index database from nix-community. Wired into the
    # HM module list below so `programs.nix-index.enable = true` (in
    # foundation.nix) reads its index from the input rather than
    # building one at activation time. nixpkgs.follows keeps the
    # closure honest — we don't want a second nixpkgs evaluation just
    # for the database build.
    nix-index-database.url = "github:nix-community/nix-index-database";
    nix-index-database.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    inputs@{
      nixpkgs,
      nixpkgs-edge,
      flake-parts,
      devenv,
      home-manager,
      nix-index-database,
      ...
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      # Single source of truth for the per-system Home Manager configuration.
      # Used by `homeConfigurations` (for `home-manager switch --flake`),
      # `packages.homeActivation` (the activation derivation `dots apply`
      # invokes via nh), and `checks.home-activation` (the `nix flake check`
      # gate that fails a PR when the activation derivation can't build).
      # Factor here so extraSpecialArgs is declared once.
      mkHome =
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        home-manager.lib.homeManagerConfiguration {
          inherit pkgs;
          modules = [
            ./modules/profiles/home.nix
            nix-index-database.homeModules.nix-index
          ];
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

      perSystem =
        { system, ... }:
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

          # Doctor helpers: assert that a given set of xdg.configFile keys
          # land in the resolved Home Manager config. Each live-edit surface
          # (fish/ghostty/zellij/wezterm/nushell/starship) routes through
          # mkOutOfStoreSymlink so edits to config/<tool>/ skip Nix eval; if
          # a future refactor drops the xdg.configFile entry, the file would
          # silently fall back to the slow `programs.<tool>.extraConfig`
          # path. This gate fails the PR before that lands.
          #
          # `nix flake check` runs in pure mode where `getEnv
          # "DOTS_WORKSPACE_ROOT"` returns "", which would collapse the
          # `lib.mkIf (workspaceRoot != "")` gate inside each module and
          # mask a missing entry. Forcing workspaceRoot to a synthetic
          # value makes the body evaluate so the assertion is meaningful.
          #
          # mkSyntheticConfig is factored out of mkConfigWiredCheck so
          # multiple checks against the same module set share one HM
          # evaluation — `homeManagerConfiguration` is one of the most
          # expensive nix-eval operations in the repo, and previously
          # each profile-level check was paying the cost independently.
          mkSyntheticConfig =
            modules:
            (home-manager.lib.homeManagerConfiguration {
              inherit pkgs;
              modules = modules ++ [
                { _module.args.workspaceRoot = nixpkgs.lib.mkForce "/synthetic/dots"; }
              ];
              extraSpecialArgs = {
                inherit inputs system;
                pkgsPins.edge = nixpkgs-edge.legacyPackages.${system};
              };
            }).config;

          mkConfigWiredCheck =
            {
              name,
              evaluated,
              expectedKeys,
              fixHint,
            }:
            let
              missing = builtins.filter (k: !(evaluated.xdg.configFile ? "${k}")) expectedKeys;
            in
            pkgs.runCommand name { } (
              if missing == [ ] then
                ''
                  echo "ok: ${name} wires ${nixpkgs.lib.concatStringsSep " " expectedKeys}" > $out
                ''
              else
                ''
                  echo "${name}: missing xdg.configFile keys: ${nixpkgs.lib.concatStringsSep " " missing}" >&2
                  echo "  why: without them, edits to the corresponding config/ dir never reach ~/.config/" >&2
                  echo "  fix: ${fixHint}" >&2
                  exit 1
                ''
            );

          # The pillar-default profile imports fish/ghostty/zellij plus
          # foundation (starship). Anything pillar-conditional that
          # *isn't* a default (e.g., wezterm, nushell) needs its own
          # minimal HM eval so the module is loaded regardless of pillar
          # selection. The nix-index-database HM module rides alongside
          # the profile because foundation.nix references its options
          # (programs.nix-index.*) — the synthetic eval would otherwise
          # report them as unknown.
          profileModules = [
            ./modules/profiles/home.nix
            nix-index-database.homeModules.nix-index
          ];
          soloModule = path: [
            path
            {
              home.username = "synthetic";
              home.homeDirectory = "/tmp/synthetic";
              home.stateVersion = "26.05";
            }
          ];

          # Shared evaluation for every check that asserts against the
          # default-pillar profile. Computed once, queried four times.
          profileEvaluated = mkSyntheticConfig profileModules;
        in
        {
          devShells.default =
            if haveDevenvRoot then
              devenv.lib.mkShell {
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

            # Live-edit symlink wiring. Each tool that surfaces its
            # config via mkOutOfStoreSymlink has to declare the matching
            # xdg.configFile entry; without it, the binary lands but the
            # imported config/<tool>/ tree never reaches ~/.config/.
            # That regression was originally reported against fish ("fish
            # was selected but did not end up installed" — abbreviations,
            # functions, conf.d entries missing). The same vector exists
            # for every live-edit surface we add; these gates fail a PR
            # the moment a refactor drops one of the entries.
            #
            # The fish module symlinks per-subtree (HM owns config.fish
            # itself), so we assert conf.d + functions; the others use a
            # single key per tool.
            fish-config-wired = mkConfigWiredCheck {
              name = "fish-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [
                "fish/conf.d"
                "fish/functions"
              ];
              fixHint = "modules/home/shells/fish.nix must declare xdg.configFile.\"fish/conf.d\" and \"fish/functions\" via mkOutOfStoreSymlink";
            };

            ghostty-config-wired = mkConfigWiredCheck {
              name = "ghostty-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [ "ghostty" ];
              fixHint = "modules/home/terminals/ghostty.nix must declare xdg.configFile.\"ghostty\" via mkOutOfStoreSymlink";
            };

            zellij-config-wired = mkConfigWiredCheck {
              name = "zellij-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [ "zellij/config.kdl" ];
              fixHint = "modules/home/multiplexers/zellij.nix must declare xdg.configFile.\"zellij/config.kdl\" via mkOutOfStoreSymlink";
            };

            starship-config-wired = mkConfigWiredCheck {
              name = "starship-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [ "starship.toml" ];
              fixHint = "modules/home/foundation.nix must declare xdg.configFile.\"starship.toml\" via mkOutOfStoreSymlink";
            };

            nvim-config-wired = mkConfigWiredCheck {
              name = "nvim-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [ "nvim" ];
              fixHint = "modules/home/editor.nix must declare xdg.configFile.\"nvim\" via mkOutOfStoreSymlink";
            };

            opencode-config-wired = mkConfigWiredCheck {
              name = "opencode-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [ "opencode" ];
              fixHint = "modules/home/opencode.nix must declare xdg.configFile.\"opencode\" via mkOutOfStoreSymlink";
            };

            lazygit-config-wired = mkConfigWiredCheck {
              name = "lazygit-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [ "lazygit/config.yml" ];
              fixHint = "modules/home/lazygit.nix must declare xdg.configFile.\"lazygit/config.yml\" via mkOutOfStoreSymlink";
            };

            herdr-config-wired = mkConfigWiredCheck {
              name = "herdr-config-wired";
              evaluated = profileEvaluated;
              expectedKeys = [ "herdr/config.toml" ];
              fixHint = "modules/home/herdr.nix must declare xdg.configFile.\"herdr/config.toml\" via mkOutOfStoreSymlink";
            };

            # Wezterm and nushell are pillar-conditional and not the
            # defaults (terminal=ghostty, shell=fish), so the profile-
            # level synthetic doesn't import them. Each gets its own
            # minimal HM eval — same regression vector, different
            # load path; can't share evaluations because each loads a
            # different module.
            wezterm-config-wired = mkConfigWiredCheck {
              name = "wezterm-config-wired";
              evaluated = mkSyntheticConfig (soloModule ./modules/home/terminals/wezterm.nix);
              expectedKeys = [ "wezterm/wezterm.lua" ];
              fixHint = "modules/home/terminals/wezterm.nix must declare xdg.configFile.\"wezterm/wezterm.lua\" via mkOutOfStoreSymlink";
            };

            nushell-config-wired = mkConfigWiredCheck {
              name = "nushell-config-wired";
              evaluated = mkSyntheticConfig (soloModule ./modules/home/shells/nushell.nix);
              expectedKeys = [ "nushell/config.nu" ];
              fixHint = "modules/home/shells/nushell.nix must declare xdg.configFile.\"nushell/config.nu\" via mkOutOfStoreSymlink";
            };
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
