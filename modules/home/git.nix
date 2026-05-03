{ pkgs, lib, identity, ... }: {
  # 12-Factor: configuration is in code; identity is an external resource.
  # The repo provides the binary and global behavior. Name/email arrive
  # from ~/.config/dots/identity.nix on the host (gitignored).
  programs.git = {
    enable = true;
    package = pkgs.git;
    userName  = identity.name;
    userEmail = identity.email;

    extraConfig = {
      init.defaultBranch = "main";
      pull.rebase = true;
      push.autoSetupRemote = true;
      rebase.autosquash = true;
      diff.algorithm = "histogram";
      merge.conflictStyle = "zdiff3";
      core.fsmonitor = true;
      fetch.prune = true;
    };

    aliases = {
      st = "status -sb";
      lg = "log --oneline --graph --decorate --all";
      co = "checkout";
      cm = "commit -m";
    };
  };
}
