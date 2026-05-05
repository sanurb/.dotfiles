# Brew
# dots: replaced hardcoded "/opt/homebrew" with detection because Apple-Silicon-only path; Intel mac uses /usr/local; Linuxbrew uses /home/linuxbrew/.linuxbrew. Skip entirely if no brew prefix exists on this machine.
set -l brew_prefix
for candidate in /opt/homebrew /usr/local /home/linuxbrew/.linuxbrew
    if test -x $candidate/bin/brew
        set brew_prefix $candidate
        break
    end
end

if test -n "$brew_prefix"
    set --global --export HOMEBREW_PREFIX "$brew_prefix"
    set --global --export HOMEBREW_CELLAR "$brew_prefix/Cellar"
    set --global --export HOMEBREW_REPOSITORY "$brew_prefix"
    fish_add_path --global --move --path "$brew_prefix/bin" "$brew_prefix/sbin"
    if test -n "$MANPATH[1]"
        set --global --export MANPATH '' $MANPATH
    end
    if not contains "$brew_prefix/share/info" $INFOPATH
        set --global --export INFOPATH "$brew_prefix/share/info" $INFOPATH
    end
end
