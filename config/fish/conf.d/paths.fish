# User paths (use $HOME for portability across machines)
fish_add_path "$HOME/.dotfiles"
fish_add_path "$HOME/.local/bin"
fish_add_path "$HOME/.opencode/bin"
fish_add_path "$HOME/.config/opencode/scripts"
# dots: replaced unconditional fish_add_path "/Applications/Ghostty.app/..." with a -d guard because the path is macOS-only and only present when Ghostty.app is installed.
if test -d /Applications/Ghostty.app/Contents/MacOS
    fish_add_path "/Applications/Ghostty.app/Contents/MacOS"
end
