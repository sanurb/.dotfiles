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

  # direnv + fzf + proto live here because they're persona-agnostic
  # baseline ergonomics. Proto is the runtime version manager: per
  # ADR-0008 it owns Go/Bun/Node/Rust/etc., and AGENTS.md commits to
  # "Proto's shims are first in $PATH" — that contract requires proto
  # the binary AND its shim directories on every shell's PATH,
  # regardless of which persona shell the user picked.
  home.packages = with pkgs; [ direnv fzf proto ];

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
