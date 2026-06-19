{ pkgs, lib, ... }:
{
  # 12-Factor: Explicit Dependencies. Everything the dev environment needs
  # is declared here; nothing is assumed from the host. Language runtimes
  # (Go, etc.) are deliberately NOT enabled via devenv's `languages.*`
  # modules — proto owns those, sourced from .prototools at the repo root.
  packages =
    with pkgs;
    [
      # Toolchain orchestration — Nix installs proto; proto installs
      # everything else from .prototools (moon, go, etc.). Keeping moon
      # here too would be "double-wrapping" — and as of May 2026 nixpkgs,
      # `moon` fails to build from source against the current Rust
      # toolchain (sdd-4.1.1 lifetime bug). Proto's prebuilt binary
      # bypasses that entirely.
      proto
      direnv # required for shell integration; doctor checks for it

      # Realization-layer drivers. nh and nom sit at distinct layers and are
      # composed via $PATH, never selected against each other:
      #   - nh  : lifecycle driver. `dots deploy` shells out to `nh home switch`,
      #           which invokes `activationPackage` directly via `nix build` and
      #           execs the resulting `./activate` script. No `home-manager` CLI
      #           required — verified against nh-4.3.x source at
      #           crates/nh-home/src/home.rs (line 223:
      #           Command::new(target_profile.join("activate")); no
      #           Command::new("home-manager") anywhere in the crate).
      #   - nom : build renderer. nh delegates automatically when nom is on $PATH;
      #           absence falls back to plain `nix build` output, no crash.
      # Floor: nh >= 4.3.0. The 4.3.0 release introduced
      # `NH_SHOW_ACTIVATION_LOGS` / `--show-activation-logs` alongside a
      # breaking change that hides activation output by default. The deploy
      # task sets the env var so failures stay legible; the doctor enforces
      # the floor.
      nh
      nix-output-monitor

      # Terminal stack. Ghostty has no aarch64-darwin nixpkgs build
      # (upstream needs Xcode/SwiftPM); on macOS install via Homebrew —
      # the Home Manager-projected config in modules/terminal.nix works
      # for either source. Linux gets it from nixpkgs.
      zellij

      # Charm CLI dependencies that apps/cli shells out to.
      gum

      # TUI demo recorder. Renders docs/demos/*.tape into GIFs that the
      # README links to; also drives manual visual regression for the
      # wizard's screen primitive.
      vhs

      # Release tooling — declared so `goreleaser check` and snapshot builds
      # work inside the dev shell without a host install. cosign and syft are
      # invoked by the GoReleaser pipeline (signing, SBOM); having them on
      # PATH locally lets a maintainer reproduce a snapshot end-to-end.
      goreleaser
      cosign
      syft

      # Day-to-day fast tooling.
      fd
      ripgrep
      bat
      eza
      jq
      git

      # CI gates — repo-wide static checkers wired through Moon as
      # `root:lint`. Each tool owns one concern:
      #   - actionlint : GitHub Actions YAML semantics (+ shellcheck on `run:` blocks)
      #   - zizmor     : GH Actions security (template injection, untrusted input,
      #                  cache poisoning). Complements actionlint, doesn't replace.
      #   - pinact     : asserts third-party actions are pinned to commit SHAs.
      # Nix linters (deadnix, statix) come for free via languages.nix.enable.
      actionlint
      zizmor
      pinact

      # Language Servers — declarative, pinned to the same nixpkgs hash as
      # the rest of the environment. "Declare, don't script": every binary
      # the workspace needs is materialized by entering the directory; no
      # imperative install step is permitted. The Go CLI's `dots doctor`
      # asserts these are reachable on $PATH and refuses deploy on miss.
      gopls # Go
      rust-analyzer # Rust
      vtsls # TypeScript/JavaScript (Volar-based)
      typescript # tsc — vtsls relies on it
      typescript-language-server
      vscode-langservers-extracted # HTML / CSS / JSON / ESLint
      lua-language-server # Lua (nvim config)
      nixd # Nix
    ]
    ++ lib.optionals pkgs.stdenv.isLinux [
      ghostty # Linux-only via nixpkgs; macOS uses Homebrew
    ];

  # Nix tooling is fine — proto does not manage Nix.
  languages.nix.enable = true;

  # First-class shell integrations.
  starship.enable = true;
  difftastic.enable = true;

  # Unified, declarative formatting. Treefmt is the only entrypoint;
  # every formatter binary is sourced from the same nixpkgs hash as
  # the rest of the shell, so a host `gofumpt` cannot disagree with CI.
  # Inline config (no standalone treefmt.toml) keeps package set and
  # formatter config in one source of truth — a TOML at the repo root
  # would split authority and let the two drift.
  #
  # Hooks live in .moon/workspace.yml under `vcs.hooks`, not here.
  # devenv's `git-hooks` module is a second hook manager (a fork of
  # pre-commit.com) — keeping it alongside Moon's vcs hooks would mean
  # two binaries racing to install .git/hooks/pre-commit, two caches,
  # and two places a contributor has to look to learn what runs on
  # commit. Toolchain Collapse: Moon already hashes inputs and gates
  # tasks; routing hooks through `moon run` reuses that machinery.
  treefmt = {
    enable = true;
    config = {
      projectRootFile = "flake.nix";

      # Tool-managed and machine-generated files are off-limits to every
      # formatter — reformatting them either fights another writer
      # (lazy.nvim, npm, Nix) or adds churn that obscures real diffs.
      settings.global.excludes = [
        "config/nvim/lazy-lock.json"
        "config/nvim/.undodir/**"
        "config/opencode/package-lock.json"
        "flake.lock"
        "*.lock"
        ".moon/cache/**"
        ".devenv/**"
        ".direnv/**"
      ];

      programs.gofumpt.enable = true;

      # nixfmt is the RFC-166 official Nix formatter; nixpkgs-fmt is
      # archived. nixpkgs ≥ 25.11 ships nixfmt as the default and the
      # `nixfmt-rfc-style` alias is a deprecation shim. Use `nixfmt`.
      programs.nixfmt.enable = true;

      # dprint covers the Markdown / JSON / TOML / YAML surface that
      # native Go/Nix formatters don't touch. Wasm plugins are pinned
      # via nixpkgs (pkgs.dprint-plugins.getPluginList) so the plugin
      # set is hermetic — no URL fetch from inside the Nix sandbox.
      # Pretty YAML (g-plane) is anchor-aware, which matters since
      # GitHub Actions enabled YAML anchors in Sept 2025.
      programs.dprint = {
        enable = true;
        includes = [
          "*.md"
          "*.json"
          "*.jsonc"
          "*.toml"
          "*.yaml"
          "*.yml"
        ];
        settings = {
          lineWidth = 100;
          indentWidth = 2;
          # dprint self-traverses (it does not consume treefmt's file
          # list), so treefmt's `global.excludes` never reaches it. Its
          # own `excludes` is the only thing that keeps it out of nvim's
          # machine-generated undo/context dir, whose %-escaped *.md files
          # carry invalid UTF-8 and abort the whole `treefmt:run` task.
          excludes = [
            "config/nvim/.undodir/**"
            "config/nvim/lazy-lock.json"
            "config/opencode/package-lock.json"
          ];
          # Preserve author line breaks for ADRs / READMEs / SKILL.md;
          # "never" would flatten every paragraph to one long line and
          # "always" would re-flow prose to fit lineWidth (also bad).
          markdown.textWrap = "maintain";
          json = { };
          toml = { };
          yaml = { };
          plugins = pkgs.dprint-plugins.getPluginList (
            plugins: with plugins; [
              dprint-plugin-markdown
              dprint-plugin-json
              dprint-plugin-toml
              g-plane-pretty_yaml
            ]
          );
        };
      };
    };
  };

  env = {
    EDITOR = "nvim";
    PAGER = "less";
  };

  # PROTO_HOME and PATH are set here, not in `env`, because devenv's
  # static env block does NOT shell-expand `$DEVENV_ROOT`. enterShell
  # runs as bash and expands variables correctly. Proto's shims dir
  # must be on PATH for Moon's task subprocesses (which inherit env
  # from this shell) to resolve go/moon/node/etc.
  enterShell = ''
    export PROTO_HOME="$DEVENV_ROOT/.proto"
    # ~/.cargo/bin is where rustup (proto's rust backend) drops rustc/cargo;
    # it's not a proto shim, so we add it explicitly. Otherwise the doctor's
    # rustc probe fails even though `proto use` succeeded.
    export PATH="$PROTO_HOME/shims:$PROTO_HOME/bin:$HOME/.cargo/bin:$PATH"
    proto use --quiet 2>/dev/null || true
  '';
}
