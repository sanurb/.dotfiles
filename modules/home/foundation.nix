{ config, pkgs, lib, ... }: {
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

  # Starship — config held as a TOML asset and projected as a read-only
  # symlink into ~/.config/starship.toml. Editing the live file is a
  # no-op; the only path to change is editing the asset and re-deploying.
  # Note: when zsh is the chosen shell, modules/shells/zsh.nix sets
  # programs.starship.enableZshIntegration = false so Powerlevel10k owns
  # the prompt — starship still ships its config but doesn't bind.
  programs.starship.enable = true;
  xdg.configFile."starship.toml".source = ./assets/starship.toml;

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

  # bottom — persona-agnostic system monitor, pure TUI. Defaults tuned
  # for triage rather than glanceable graphs: mem_as_value surfaces
  # absolute MB/GB (a "73%" reading on a 64 GB machine tells you
  # nothing; "23.4 GB" tells you the leak), group_processes collapses
  # the N node/python/rust-analyzer workers a dev box spawns, and
  # default_widget_type lands on the process table — what users
  # actually open btm to see. Colors left to auto-detect so we respect
  # the terminal palette instead of fighting it; override per-machine
  # if the default clashes.
  programs.bottom = {
    enable = true;
    settings = {
      flags = {
        rate = 1000;
        mem_as_value = true;
        group_processes = true;
        tree = false;
        temperature_type = "celsius";
        time_delta = 15000;
        default_widget_type = "proc";
        hide_table_gap = true;
      };

      colors = { };

      processes = {
        columns = [
          "PID"
          "Name"
          "CPU%"
          "Mem%"
          "R/s"
          "W/s"
          "State"
        ];
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
  # shell session reaches for: bat (cat with syntax + paging), fd
  # (find with sane defaults), sd (sed for the single-pass-replace
  # case), hyperfine (statistical command benchmarks). Persona-
  # agnostic, so they sit alongside direnv/fzf/proto rather than in
  # any per-shell module.
  home.packages = with pkgs; [ direnv fzf proto bat fd sd hyperfine eza ];

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
    shellAliases.nix = "nom";
  };

  programs.zsh = {
    enable = true;
    shellAliases.nix = "nom";
  };
}
