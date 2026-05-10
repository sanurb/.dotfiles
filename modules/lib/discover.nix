{ lib }:
let
  # discoverDir :: Path -> AttrSet
  #
  # Walks `path` recursively. Each `<name>.nix` regular file becomes a leaf
  # `<name> = <path>`; each subdirectory becomes a nested attrset produced by
  # the same walk. Subdirectories that contain no `.nix` files at any depth
  # are dropped — this skips asset trees (`assets/starship.toml`, etc.) so
  # downstream lookups don't see empty namespaces.
  #
  # The result mirrors the directory layout exactly:
  #
  #   modules/home/foundation.nix         -> .foundation        :: Path
  #   modules/home/shells/fish.nix        -> .shells.fish       :: Path
  #   modules/home/shells/zsh.nix         -> .shells.zsh        :: Path
  #
  # so `homeModules.shells.${pillars.shell}` is the canonical lookup form
  # in modules/profiles/home.nix.
  discoverDir =
    path:
    let
      entries = builtins.readDir path;

      isNixFile = name: type: type == "regular" && lib.hasSuffix ".nix" name;

      moduleEntries = lib.mapAttrs' (
        name: _: lib.nameValuePair (lib.removeSuffix ".nix" name) (path + "/${name}")
      ) (lib.filterAttrs isNixFile entries);

      subDirs = lib.filterAttrs (_: type: type == "directory") entries;

      subDirEntries = lib.mapAttrs (name: _: discoverDir (path + "/${name}")) subDirs;

      nonEmptySubDirs = lib.filterAttrs (_: v: v != { }) subDirEntries;
    in
    moduleEntries // nonEmptySubDirs;
in
{
  inherit discoverDir;
}
