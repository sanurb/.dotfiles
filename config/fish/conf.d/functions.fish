function fvim
    if test (count $argv) -eq 0
        fd -H -t f | fzf --header "Open File in Vim" --preview "cat {}" | xargs nvim
    else
        set -l query (string join " " $argv)
        fd -H -t f | fzf --header "Open File in Vim" --preview "cat {}" -q "$query" | xargs nvim
    end
end

function vim
    if test (count $argv) -eq 0
        nvim .
    else
        nvim $argv
    end
end

function vi
    if test (count $argv) -eq 0
        nvim .
    else
        nvim $argv
    end
end

# dots: removed the `localcode` function because it hardcoded /Users/$USER/Code/personal/opencode/packages/opencode, a developer-machine-specific checkout that no other user has.
