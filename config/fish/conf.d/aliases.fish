alias c 'clear'
alias code 'vim'
alias grep 'grep --color=auto --exclude-dir={.bzr,CVS,.git,.hg,.svn,.idea,.tox}'
alias ks 'tmux kill-server'
alias pbc 'pbcopy'
alias pbp 'pbpaste'
alias pn 'pnpm'
alias oc 'opencode'
# Disable automatic completion generation for oc to avoid errors
complete -c oc -e
alias scratch 'nvim -c "setlocal buftype=nofile"'
alias vimdiff 'nvim -d'
alias wr 'wrangler'
# dots: removed alias `lc 'localcode'` because the localcode function it pointed to was machine-specific and has been removed.

# zoxide-backed parent traversal — `z ..` honors zoxide's frecency
# rules, so `..` here is functionally `cd ..` plus rank bookkeeping.
alias .. 'z ..'
alias ... 'z ../..'
alias .3 'z ../../..'
alias .4 'z ../../../..'
alias .5 'z ../../../../..'

# Single-letter shortcuts for the tools foundation.nix installs.
alias q 'exit'
alias v 'nvim'
alias f 'fd'
alias r 'rg --smart-case'
alias b 'bat'
alias s 'sd'
alias bench 'hyperfine'
alias reload 'exec $SHELL -l'

# eza variants — concise listing, full -ahlF detail with git status,
# and a depth-2 tree. Icons + group-directories-first match the
# default we expect every shell to ship with.
alias e 'eza -1 --icons --color=always --group-directories-first'
alias ee 'eza -ahlF --icons --color=always --group-directories-first --git --color-scale'
alias et 'eza --tree --level=2 --icons'
