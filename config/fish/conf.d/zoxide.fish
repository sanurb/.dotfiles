# zoxide is initialized by Home Manager — see
# programs.zoxide.enableFishIntegration in modules/home/shells/fish.nix.
# Don't re-init here; HM injects the eval into ~/.config/fish/config.fish
# which fish loads after this conf.d, so a manual `zoxide init fish | source`
# would run twice (once wasted) on every shell start.
#
# `zi` (interactive picker) is wired by zoxide itself. We add `j` purely
# for shorter muscle memory.
alias j 'zi'
