{ pkgs, ... }: {
  # procs — modern `ps` replacement with colorized columns, tree view,
  # search/sort, and pager integration. Sibling to bat/delta in the
  # "richer terminal output" satellite tier; never replaces ps because
  # scripts that parse ps output (init systems, ad-hoc shell pipelines)
  # rely on its POSIX columns staying stable.
  #
  # No `programs.procs` HM module exists; direct package install. User
  # config (~/.config/procs/config.toml) is optional and project-style
  # — left unmanaged here so a host can drop one in without a flake
  # rebuild.
  home.packages = [ pkgs.procs ];
}
