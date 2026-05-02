{
  description = "dots";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    devenv.url = "github:cachix/devenv";
    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, devenv, home-manager, ... } @ inputs:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system);

      mkHome = system: home-manager.lib.homeManagerConfiguration {
        pkgs = nixpkgs.legacyPackages.${system};
        modules = [ ./apps/dotfiles/home.nix ];
        extraSpecialArgs = { inherit inputs system; };
      };
    in {
      devShells = forAllSystems (system: {
        default = devenv.lib.mkShell {
          inherit inputs;
          pkgs = nixpkgs.legacyPackages.${system};
          modules = [ ./devenv.nix ];
        };
      });

      # Multi-system activation entrypoint. Moon invokes:
      #   nix build --impure --accept-flake-config .#homeActivation
      #   ./.result-home/activate
      packages = forAllSystems (system: {
        homeActivation = (mkHome system).activationPackage;
        home-manager = home-manager.packages.${system}.default;
      });

      homeConfigurations = forAllSystems (system: {
        default = mkHome system;
      });

      formatter = forAllSystems (system:
        nixpkgs.legacyPackages.${system}.nixpkgs-fmt
      );
    };
}
