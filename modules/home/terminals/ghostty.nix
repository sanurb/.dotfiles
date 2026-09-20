{
  pkgs,
  lib,
  workspaceRoot,
  ...
}:
let
  coreutils = lib.getBin pkgs.coreutils;
in
{
  # Ghostty — config managed as a plain text file so it remains portable
  # to a non-Nix host (just copy the file). The upstream `pkgs.ghostty`
  # is Linux-only as of nixpkgs 26.05 (Zig + macOS toolchain mismatch),
  # so on darwin the .app is installed from a HM activation script via
  # Homebrew Cask. This honors the dots promise that selecting a
  # terminal in the wizard installs it — even when the install happens
  # outside Nix on macOS.
  #
  # Startup-critical config deliberately bypasses xdg.configFile. Home
  # Manager's mkOutOfStoreSymlink still routes through /nix/store before
  # reaching the workspace, so Ghostty loses its config during a delayed Nix
  # APFS mount — the same window in which a Nix-backed login shell is absent.
  # Direct links keep the config, themes, and shaders available independently
  # of /nix while retaining the live-edit workflow.
  #
  # Ghostty 1.2.3 made config.ghostty canonical. On macOS we use its native
  # Application Support location so `ghostty +edit-config` opens the managed
  # file instead of creating an empty native file that appears to have wiped
  # the XDG config.
  home.activation.linkGhosttyConfig = lib.mkIf (workspaceRoot != "") (
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      ghostty_source=${lib.escapeShellArg "${workspaceRoot}/config/ghostty"}

      prepare_ghostty_config_dir() {
        local config_dir="$1"
        local link_target

        if [ -L "$config_dir" ]; then
          link_target="$(${coreutils}/bin/readlink "$config_dir" 2>/dev/null || true)"
          case "$link_target" in
            /nix/store/*|"$ghostty_source")
              run ${coreutils}/bin/rm -- "$config_dir"
              ;;
            *)
              echo "ghostty: preserving unmanaged link $config_dir -> $link_target" >&2
              return 1
              ;;
          esac
        elif [ -e "$config_dir" ] && [ ! -d "$config_dir" ]; then
          echo "ghostty: $config_dir exists but is not a directory" >&2
          return 1
        fi
        if [ ! -e "$config_dir" ]; then
          run ${coreutils}/bin/mkdir -p "$config_dir"
        fi
      }

      link_ghostty_config_path() {
        local source_path="$1"
        local destination_path="$2"

        if [ -L "$destination_path" ]; then
          run ${coreutils}/bin/rm -- "$destination_path"
        elif [ -e "$destination_path" ]; then
          # Ghostty creates an empty native config on first edit. It contains
          # no user data and is safe to replace with the managed config.
          if [ -f "$destination_path" ] && [ ! -s "$destination_path" ]; then
            run ${coreutils}/bin/rm -- "$destination_path"
          else
            echo "ghostty: preserving unmanaged path $destination_path" >&2
            return
          fi
        fi
        run ${coreutils}/bin/ln -s "$source_path" "$destination_path"
      }

      # Themes and relative shader paths are always resolved through XDG on
      # macOS too, even when the main config comes from Application Support.
      ghostty_xdg_dir="$HOME/.config/ghostty"
      if prepare_ghostty_config_dir "$ghostty_xdg_dir"; then
        link_ghostty_config_path "$ghostty_source/shaders" "$ghostty_xdg_dir/shaders"
        link_ghostty_config_path "$ghostty_source/themes" "$ghostty_xdg_dir/themes"
      fi

      ${
        if pkgs.stdenv.hostPlatform.isDarwin then
          ''
            ghostty_native_dir="$HOME/Library/Application Support/com.mitchellh.ghostty"
            if prepare_ghostty_config_dir "$ghostty_native_dir"; then
              link_ghostty_config_path "$ghostty_source/config" "$ghostty_native_dir/config.ghostty"
            fi
          ''
        else
          ''
            if [ -d "$ghostty_xdg_dir" ]; then
              link_ghostty_config_path "$ghostty_source/config" "$ghostty_xdg_dir/config.ghostty"
            fi
          ''
      }
    ''
  );

  # Activation hook: ensure Ghostty.app exists on macOS. Idempotent —
  # checks both /Applications and ~/Applications before invoking brew.
  # The lib.mkIf gate keeps the script out of Linux closures.
  #
  # `nh home switch` doesn't source `brew shellenv`, so PATH at hook
  # time may lack `/opt/homebrew/bin`. We probe the canonical install
  # paths so the hook still works when apply is launched from a shell
  # (or devenv subshell) where shellenv hasn't run.
  home.activation.installGhostty = lib.mkIf pkgs.stdenv.hostPlatform.isDarwin (
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      if [ -d "/Applications/Ghostty.app" ] || [ -d "$HOME/Applications/Ghostty.app" ]; then
        $VERBOSE_ECHO "ghostty: already installed; skipping brew cask"
      else
        BREW=
        for c in $(command -v brew 2>/dev/null) /opt/homebrew/bin/brew /usr/local/bin/brew; do
          [ -x "$c" ] && { BREW=$c; break; }
        done
        if [ -n "$BREW" ]; then
          $VERBOSE_ECHO "ghostty: installing via brew cask ($BREW)"
          run "$BREW" install --cask ghostty
        else
          echo "ghostty: brew not found — Ghostty.app was NOT installed." >&2
          echo "  fix: install Homebrew, then rerun \`dots apply\`:" >&2
          echo "       /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"" >&2
        fi
      fi
    ''
  );
}
