{
  pkgs,
  lib,
  identity,
  ...
}:
let
  # Optional identity fields. The base contract is { name; email; } —
  # everything below activates only when the host's identity.nix
  # provides the corresponding key, so a public-safe checkout without a
  # signing key still produces working git commits (unsigned).
  signingKey = identity.signingKey or null;
  githubUser = identity.githubUser or null;
in
{
  # 12-Factor: configuration is in code; identity is an external resource.
  # The repo provides the binary and global behavior. Name/email arrive
  # from ~/.config/dots/identity.nix on the host (gitignored), as do the
  # optional signingKey and githubUser fields.
  programs.git = {
    enable = true;
    package = pkgs.git;

    # LFS for the handful of repos that ship binaries (design assets,
    # ML weights). skipSmudge stays off — we want clones to fetch
    # objects automatically; the explicit `git lfs pull` workflow is a
    # foot-gun (forgotten pulls land as broken pointer files in the
    # working tree).
    lfs = {
      enable = true;
      skipSmudge = false;
    };

    # Global ignore list. These are editor / OS turds that have no
    # business in any repo's per-project .gitignore — promoting them to
    # the global file means the next person who clones doesn't have to
    # re-add them, and we don't pollute upstream PRs to other people's
    # projects with our tooling noise.
    ignores = [
      ".DS_Store"
      ".idea"
      ".vscode"
      ".scratch"
      "__scratch"
      "*.swp"
    ];

    # Commit signing. Only flips on when identity.signingKey is set on
    # the host — keeps the module safe to evaluate on a fresh clone
    # before the user has provisioned an SSH signing key. SSH-format
    # signing avoids the GPG agent entirely; the verification side
    # (allowed_signers) lives outside HM since it's per-repo.
    signing = lib.mkIf (signingKey != null) {
      format = "ssh";
      key = signingKey;
      signByDefault = true;
    };

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
      rebase.autoStash = true;
      diff.algorithm = "histogram";
      merge.conflictStyle = "zdiff3";
      fetch.prune = true;
      # Remember resolutions so the same conflict on a re-rebase /
      # re-merge resolves itself silently.
      rerere.enabled = true;

      # core.* — editor matches the rest of the workspace (nvim is the
      # default editor in modules/home/editor.nix). fileMode off because
      # macOS+Linux dual-boot setups churn the executable bit on every
      # checkout otherwise. ignorecase off because case-insensitive
      # filesystems (default on macOS) make git treat Foo.ts and foo.ts
      # as the same path — a bug magnet on cross-platform repos.
      # fsmonitor is the built-in watcher; large repos (>10k files) feel
      # noticeably faster with it on, small repos see no harm.
      core.editor = "nvim";
      core.fileMode = false;
      core.ignorecase = false;
      core.fsmonitor = true;

      # github.user is gh + hub's lookup key; only set it when the host
      # provides one so we don't pin a public-safe default.
      github = lib.mkIf (githubUser != null) { user = githubUser; };

      alias = {
        st = "status -sb";
        lg = "log --oneline --graph --decorate --all";
        co = "checkout";
        cm = "commit -m";
        # Open the current repo's GitHub page in a browser. Strips the
        # trailing `.git` and rewrites SSH remotes to https — the
        # rewrite is idempotent against already-https remotes.
        open = "!f() { open \"$(git config --get remote.origin.url | sed -E 's,\\.git$,,;s,git@github\\.com:,https://github.com/,')\"; }; f";
      };
    };
  };
}
