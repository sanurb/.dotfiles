# Vite+ bin (https://viteplus.dev)
# dots: replaced unguarded `source "$HOME/.vite-plus/env.fish"` with a -f test because Vite+ is optional and `source` on a missing file errors loudly during shell init.
if test -f "$HOME/.vite-plus/env.fish"
    source "$HOME/.vite-plus/env.fish"
end
