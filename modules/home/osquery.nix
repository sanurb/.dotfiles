{ pkgs, lib, ... }:
{
  # osquery — SQL-as-introspection over OS state (processes, sockets,
  # users, launchd/systemd jobs, package managers, etc.). Ships
  # `osqueryi` (interactive shell) and `osqueryd` (daemon); we install
  # both via the package but don't enable the daemon — running osqueryd
  # as a long-lived service is a system-level decision out of scope for
  # a Home Manager profile. `osqueryi` works without it for ad-hoc
  # queries.
  #
  # Darwin note: nixpkgs `osquery` lists Linux-only meta.platforms (eval
  # refuses on aarch64-darwin / x86_64-darwin), so the install is gated
  # to Linux. On macOS the supported path is the official pkg from
  # osquery.io — a one-time manual install per host, same shape as
  # granting Full Disk Access (which osquery also needs on macOS for
  # the full `processes` / `system_info` table fidelity).
  home.packages = lib.optional pkgs.stdenv.isLinux pkgs.osquery;
}
