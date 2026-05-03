{
  description = "dots";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    devenv.url = "github:cachix/devenv";
    # devenv's treefmt module (current versions) sources the formatter
    # set from treefmt-nix and refuses to evaluate without it. Pin and
    # follow nixpkgs so gofumpt/nixpkgs-fmt stay on the same hash as
    # the rest of the dev shell.
    treefmt-nix.url = "github:numtide/treefmt-nix";
    treefmt-nix.inputs.nixpkgs.follows = "nixpkgs";
    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, devenv, treefmt-nix, home-manager, ... } @ inputs:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system);

      mkHome = system: home-manager.lib.homeManagerConfiguration {
        pkgs = nixpkgs.legacyPackages.${system};
        modules = [ ./modules/profiles/home.nix ];
        extraSpecialArgs = { inherit inputs system; };
      };
    in {
      devShells = forAllSystems (system: {
        # `devenv.root` override unblocks `nix develop` from a non-direnv
        # session: devenv.lib.mkShell asserts a known project directory and,
        # without it, fails with "devenv was not able to determine the current
        # directory." Binding the root inline here is the upstream-recommended
        # fix for flake users who don't want to require --no-pure-eval.
        # Trade-off: the flake becomes non-portable across machines (the path
        # is baked into the eval), which is fine for a personal dotfiles repo
        # but would matter for a published flake. Reference:
        # https://devenv.sh/guides/using-with-flakes/
        default = devenv.lib.mkShell {
          inherit inputs;
          pkgs = nixpkgs.legacyPackages.${system};
          modules = [
            { devenv.root = builtins.toString ./.; }
            ./devenv.nix
          ];
        };
      });

      # Multi-system activation entrypoint. Moon invokes:
      #   nix build --impure --accept-flake-config .#homeActivation
      #   ./.result-home/activate
      packages = forAllSystems (system: {
        homeActivation = (mkHome system).activationPackage;
        home-manager = home-manager.packages.${system}.default;
      });

      # Each value must BE a homeConfiguration (carrying .activationPackage
      # directly) — `nix flake check` walks this attrset and inspects each
      # entry's .activationPackage.system. The previous shape nested under
      # `<system>.default = …`, which left .activationPackage one level too
      # deep and tripped the check. Keying by system gives:
      #   home-manager switch --flake .#aarch64-darwin
      # while Moon's deploy keeps using packages.${system}.homeActivation.
      homeConfigurations = nixpkgs.lib.genAttrs systems mkHome;

      formatter = forAllSystems (system:
        nixpkgs.legacyPackages.${system}.nixpkgs-fmt
      );
    };
}
