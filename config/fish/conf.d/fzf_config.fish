# fzf.fish runtime tuning — env vars and per-key preview commands.
# Bindings live in fish.nix interactiveShellInit (vendor conf.d
# clobbers user conf.d at startup).

set -gx FZF_DEFAULT_COMMAND 'fd --type f --hidden --follow --exclude .git'
set -gx FZF_CTRL_T_COMMAND  $FZF_DEFAULT_COMMAND
set -gx FZF_ALT_C_COMMAND   'fd --type d --hidden --follow --exclude .git'

# `bg:-1` inherits terminal background so Ghostty's blur+opacity show
# through. Hex codes mirror config/ghostty/themes/gentleman.
set -gx FZF_DEFAULT_OPTS '--height=40% --layout=reverse --border=rounded --info=inline --color=fg:#f3f6f9,bg:-1,hl:#7fb4ca,fg+:#f3f6f9,bg+:#263356,hl+:#ffe066,info:#7aa89f,prompt:#cb7c94,pointer:#e0c15a,marker:#ff8dd7,spinner:#ffe066,header:#8a8fa3,border:#263356'

set -gx FZF_CTRL_T_OPTS "--preview='bat --style=numbers --color=always --line-range=:300 {}'"
set -gx FZF_ALT_C_OPTS  "--preview='eza --tree --color=always --level=2 --icons {} | head -200'"
