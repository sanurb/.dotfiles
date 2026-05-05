# dots: bridge for home.sessionPath when ~/.config/fish is overridden
# by mkOutOfStoreSymlink. Normally home-manager writes
# ~/.config/fish/conf.d/hm-session-vars.fish (which re-exports the
# session vars including PATH), but our fish module replaces the
# whole ~/.config/fish directory with a symlink to this repo, so HM's
# generated bridge never lands. This file mirrors home.sessionPath
# from modules/home/foundation.nix; if you add a directory there,
# add it here too. The order must match so fish's $PATH agrees with
# zsh/bash $PATH.
for dir in $HOME/.proto/shims $HOME/.proto/bin $HOME/.local/bin $HOME/.cargo/bin $HOME/go/bin $HOME/.bun/bin $HOME/.deno/bin
    if test -d $dir
        if not contains $dir $PATH
            set -gx PATH $dir $PATH
        end
    end
end
