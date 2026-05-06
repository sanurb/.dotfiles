{ config, pkgs, lib, workspaceRoot, ... }: {
  # Foundation — the "air" the environment breathes. These three tools
  # (Atuin, Zoxide, Starship) are mandatory infrastructure for every
  # persona, regardless of which shell / terminal / multiplexer the user
  # picked in `dots install`. The TUI never asks about them; home.nix
  # always imports this module.
  #
  # Shell-specific integrations live in the per-shell modules under
  # modules/shells/ — they flip the corresponding `enable<Shell>Integration`
  # bools. We don't enable all of them here because home-manager will try
  # to write into config files for shells that aren't enabled.

  programs.atuin = {
    enable = true;
    settings = {
      auto_sync = false;
      sync_frequency = "0";
      update_check = false;
      style = "compact";
      inline_height = 12;
    };
  };

  programs.zoxide.enable = true;

  # nix-index — pre-built database from nix-community/nix-index-database
  # (input wired in flake.nix). Together with the comma helper enabled
  # below, an unknown command (e.g., `cowsay foo`) prints the matching
  # nix-run / `, cowsay foo` suggestion instead of "command not found".
  # No live `nix-index` build runs at activation; the input ships the
  # database pre-built for the channel.
  #
  # Per-shell command-not-found hooks (enable{Bash,Fish,Nushell,Zsh}-
  # Integration) default to true on this module, but the explicit
  # toggles live in modules/home/shells/{fish,zsh,nushell}.nix beside
  # the matching atuin/zoxide flips — same boundary. Bash has no
  # per-shell module; its hook is implicit via the default. See
  # docs/notes/nix-index-followups.md.
  programs.nix-index.enable = true;
  programs.nix-index-database.comma.enable = true;

  # ripgrep — declarative config via home-manager's `arguments` list.
  # Flags here are baked into a generated ripgreprc and exposed via
  # RIPGREP_CONFIG_PATH, so every shell sees the same defaults whether
  # invoked interactively or from scripts.
  programs.ripgrep = {
    enable = true;
    arguments = [
      # Search hidden files / directories.
      "--hidden"
      # Skip really huge binary blobs.
      "--max-filesize=5M"
    ];
  };

  # Starship — live-editable seam to config/starship/starship.toml,
  # same pattern as modules/home/editor.nix and the per-shell modules.
  # Editing the file in the repo is picked up by the next prompt render
  # (starship reads its config on each draw); no `dots apply` round-trip.
  # When workspaceRoot is empty (HM run outside `dots apply`) we skip
  # the link rather than emit a dangling pointer — starship falls back
  # to its built-in default prompt.
  # Note: when zsh is the chosen shell, modules/shells/zsh.nix sets
  # programs.starship.enableZshIntegration = false so Powerlevel10k owns
  # the prompt — starship still ships its config but doesn't bind.
  programs.starship.enable = true;
  xdg.configFile."starship.toml" = lib.mkIf (workspaceRoot != "") {
    source = config.lib.file.mkOutOfStoreSymlink
      "${workspaceRoot}/config/starship/starship.toml";
  };

  # Jujutsu — Git-compatible VCS, persona-agnostic infrastructure like
  # the rest of this cluster. No shell-hook story (pure CLI), config
  # small enough that programs.jujutsu.settings beats a live-editable
  # symlink. Identity intentionally omitted: jj falls back to
  # ~/.config/git/config for user.name / user.email when its own values
  # are unset, and `dots install` owns git identity — duplicating it
  # here would drift the two. auto-local-bookmark is non-negotiable for
  # the `jj git init --colocate` workflow that's the dominant deployment
  # in 2026; without it, branches created in git don't show up as jj
  # bookmarks. Pager pinned to ":builtin" because letting jj inherit
  # $PAGER misrenders ANSI when bat is the system pager.
  programs.jujutsu = {
    enable = true;
    settings = {
      ui = {
        default-command = "log";
        pager = ":builtin";
        diff.format = "git";
      };
      git.auto-local-bookmark = true;
      revsets.log = "@ | ancestors(immutable_heads().., 2) | trunk()";
      aliases = {
        l = [ "log" ];
        st = [ "status" ];
        d = [ "diff" ];
      };
    };
  };

  # direnv + fzf + proto live here because they're persona-agnostic
  # baseline ergonomics. Proto is the runtime version manager: per
  # ADR-0008 it owns Go/Bun/Node/Rust/etc., and AGENTS.md commits to
  # "Proto's shims are first in $PATH" — that contract requires proto
  # the binary AND its shim directories on every shell's PATH,
  # regardless of which persona shell the user picked.
  # Modern CLI ergonomics — drop-ins / superchargers for tools every
  # shell session reaches for: fd (find with sane defaults), sd (sed
  # for the single-pass-replace case), hyperfine (statistical command
  # benchmarks), eza (ls replacement). Persona-agnostic, so they sit
  # alongside direnv/fzf/proto rather than in any per-shell module.
  home.packages = with pkgs; [ direnv fzf proto fd sd hyperfine eza just watchexec xh d2 ];

  # Source of truth for $PATH additions. home.sessionPath writes
  # into ~/.config/hm-session-vars.{sh,fish}, sourced by every HM-
  # managed shell (bash/zsh via /etc/profile-style hookup; fish via
  # conf.d/hm-session-vars.fish). Any shell-rc-level PATH manipulation
  # MUST read from here — duplicating dirs in ~/.zshrc, conf.d/path.fish,
  # etc. is a maintenance trap that causes subtle ordering bugs.
  #
  # Proto (~/.proto/{shims,bin}) leads so a workspace's .prototools
  # pin wins against any system-installed Go/Node/etc. The user-level
  # package-manager directories follow because their binaries (pipx,
  # cargo install, go install, bun add -g, deno install) live outside
  # Nix and need to be reachable without the user editing ~/.zshrc by
  # hand — a workaround that fails silently when HM owns the shellrc
  # symlink.
  home.sessionPath = [
    "${config.home.homeDirectory}/.proto/shims"
    "${config.home.homeDirectory}/.proto/bin"
    "${config.home.homeDirectory}/.local/bin" # pipx, pip --user, generic
    "${config.home.homeDirectory}/.cargo/bin" # rustup, cargo install
    "${config.home.homeDirectory}/go/bin" # go install
    "${config.home.homeDirectory}/.bun/bin" # bun
    "${config.home.homeDirectory}/.deno/bin" # deno
  ];

  programs.direnv = {
    enable = true;
    nix-direnv.enable = true;

    # Global behavior. load_dotenv lets a project's .env sit alongside
    # .envrc and be picked up automatically — useful for projects that
    # ship a .env.example without insisting on an envrc shim. strict_env
    # stays off because some upstream envrc templates reference unset
    # vars; turning it on hard-fails those rather than warning. warn_
    # timeout surfaces slow envrcs at 5s, which in practice catches
    # nix-direnv cache misses early (a cold `use flake` is multi-second,
    # a warm one is sub-second — the warning is the signal that the
    # cache got blown away).
    #
    # Per-shell `enable<Shell>Integration` toggles intentionally do NOT
    # live here — same boundary as starship/atuin/zoxide. Foundation is
    # persona-agnostic, and the per-shell modules under
    # modules/home/shells/ own those flags. Bash/Zsh/Fish pick up
    # direnv via home-manager's defaults today; explicit toggles (and
    # nushell coverage) are tracked as a follow-up.
    config = {
      global = {
        load_dotenv = true;
        strict_env = false;
        warn_timeout = "5s";
      };

      # Auto-trust envrcs under our standard project roots. Anything
      # outside these prefixes still requires `direnv allow` — the
      # whitelist is a convenience for trees we already own, not a
      # blanket permission.
      whitelist.prefix = [
        "${config.home.homeDirectory}/code"
        "${config.home.homeDirectory}/projects"
        "${config.home.homeDirectory}/work"
      ];
    };
  };

  # Bash and Zsh always available — non-interactive fallback (bash)
  # and the macOS default login shell (zsh). HM-managing both means
  # session-vars (PATH including .proto/shims) and the direnv hook
  # reach the actual shell the user runs, regardless of which persona
  # shell `dots install` captured. Persona-specific opinions
  # (Powerlevel10k, fish abbreviations, etc.) layer on top in
  # modules/home/shells/<persona>.nix when that persona is selected.
  programs.bash = {
    enable = true;
    initExtra = ''
      command -v starship >/dev/null && eval "$(starship init bash)"
      command -v direnv   >/dev/null && eval "$(direnv hook bash)"
    '';
  };

  programs.zsh.enable = true;

  # Route `nix` through nom (nix-output-monitor). Declared once in
  # home.shellAliases — HM propagates it to bash, zsh, fish, and
  # nushell automatically, so per-shell modules don't need to repeat
  # it. nom is a transparent drop-in: forwards every subcommand to
  # nix and renders a live build graph for build/develop/shell/run.
  home.shellAliases.nix = "nom";
}
