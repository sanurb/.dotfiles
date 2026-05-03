{ pkgs, lib, ... }: {
  # 12-Factor: Explicit Dependencies. Everything the dev environment needs
  # is declared here; nothing is assumed from the host. Language runtimes
  # (Go, etc.) are deliberately NOT enabled via devenv's `languages.*`
  # modules — proto owns those, sourced from .prototools at the repo root.
  packages = with pkgs; [
    # Toolchain orchestration — Nix installs proto; proto installs
    # everything else from .prototools (moon, go, etc.). Keeping moon
    # here too would be "double-wrapping" — and as of May 2026 nixpkgs,
    # `moon` fails to build from source against the current Rust
    # toolchain (sdd-4.1.1 lifetime bug). Proto's prebuilt binary
    # bypasses that entirely.
    proto
    direnv # required for shell integration; doctor checks for it
    nix-output-monitor # `nom` — pretty live build graph; aliased over `nix`

    # Terminal stack. Ghostty has no aarch64-darwin nixpkgs build
    # (upstream needs Xcode/SwiftPM); on macOS install via Homebrew —
    # the Home Manager-projected config in modules/terminal.nix works
    # for either source. Linux gets it from nixpkgs.
    zellij

    # Charm CLI dependencies that apps/cli shells out to.
    gum

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
  ] ++ lib.optionals pkgs.stdenv.isLinux [
    ghostty # Linux-only via nixpkgs; macOS uses Homebrew
  ];

  # Nix tooling is fine — proto does not manage Nix.
  languages.nix.enable = true;

  # First-class shell integrations.
  starship.enable = true;
  difftastic.enable = true;

  # Unified, declarative formatting. Treefmt is the only entrypoint —
  # `gofumpt` for apps/cli Go sources, `nixpkgs-fmt` for every .nix file.
  # Both binaries are sourced from the same nixpkgs hash as the rest of
  # the shell, so the formatter set is hermetic: there is no host
  # `gofumpt` that can disagree with CI. Per SOP "Declare, Don't Script":
  # no standalone treefmt.toml — the config lives here.
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
      programs.gofumpt.enable = true;
      programs.nixpkgs-fmt.enable = true;
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
