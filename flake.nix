{
  description = "dots";

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

    treefmt-nix.url = "github:numtide/treefmt-nix";
    treefmt-nix.inputs.nixpkgs.follows = "nixpkgs";

    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = inputs@{ self, nixpkgs, nixpkgs-edge, flake-parts, devenv, home-manager, ... }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];

      # Single source of truth for the per-system Home Manager configuration.
      # Used by both `homeConfigurations` (for `home-manager switch --flake`)
      # and `packages.homeActivation` (for `moon run dotfiles:deploy`); the
      # two outputs differ only in shape (config object vs activation drv),
      # so factor here and avoid re-declaring extraSpecialArgs twice.
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

      perSystem = { system, ... }: let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        devShells.default = devenv.lib.mkShell {
          inherit inputs pkgs;
          modules = [
            # Bind the project directory so `nix develop` works from a
            # non-direnv shell. https://devenv.sh/guides/using-with-flakes/
            { devenv.root = builtins.toString ./.; }
            ./devenv.nix
          ];
        };

        packages = {
          homeActivation = (mkHome system).activationPackage;
          home-manager = home-manager.packages.${system}.default;
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
