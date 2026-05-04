{ pkgs, lib, identity, ... }: {
  # 12-Factor: configuration is in code; identity is an external resource.
  # The repo provides the binary and global behavior. Name/email arrive
  # from ~/.config/dots/identity.nix on the host (gitignored).
  programs.git = {
    enable = true;
    package = pkgs.git;

    # home-manager unified git configuration under programs.git.settings
    # in 2026.x — userName / userEmail / extraConfig / aliases all moved
    # under settings.{user.name,user.email,*,alias}. Eval emits warnings
    # when the old names are still used; the deploy on the user's host
    # surfaced six of them. Migrating to the new names silences eval and
    # protects against the imminent removal of the deprecated aliases.
    settings = {
      user.name = identity.name;
      user.email = identity.email;

      init.defaultBranch = "main";
      pull.rebase = true;
      push.autoSetupRemote = true;
      rebase.autosquash = true;
      diff.algorithm = "histogram";
      merge.conflictStyle = "zdiff3";
      core.fsmonitor = true;
      fetch.prune = true;

      alias = {
        st = "status -sb";
        lg = "log --oneline --graph --decorate --all";
        co = "checkout";
        cm = "commit -m";
      };
    };
  };
}
