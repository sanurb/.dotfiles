# dots: bridge for home.sessionPath when ~/.config/fish is overridden
# by mkOutOfStoreSymlink. Normally home-manager writes
# ~/.config/fish/conf.d/hm-session-vars.fish (which re-exports the
# session vars including PATH), but our fish module replaces the
# whole ~/.config/fish directory with a symlink to this repo, so HM's
# generated bridge never lands. This file mirrors home.sessionPath
# from modules/home/foundation.nix; if you add a directory there,
# add it here too. The order must match so fish's $PATH agrees with
# zsh/bash $PATH.
#
# Nix profile bins lead the list because fish-as-login-shell on macOS never
# sources /etc/profile.d/nix.sh (bash-only), and HM's generated config.fish
# emits absolute /nix/store/.../<tool> init fish | source lines whose output
# bare-calls `atuin`, `zoxide`, etc. — those bare calls need the profile bins
# already on PATH at conf.d time (conf.d runs before config.fish).
for dir in /nix/var/nix/profiles/default/bin $HOME/.nix-profile/bin $HOME/.proto/shims $HOME/.proto/bin $HOME/.local/bin $HOME/.cargo/bin $HOME/go/bin $HOME/.bun/bin $HOME/.deno/bin
    if test -d $dir
        if not contains $dir $PATH
            set -gx PATH $dir $PATH
        end
    end
end
