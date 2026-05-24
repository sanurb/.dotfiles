# Route `nix` build-like subcommands through `nom` (nix-output-monitor)
# for live build-graph rendering, but fall through to real `nix` for
# everything else. nom only wraps `build`/`shell`/`develop` — calling
# `nom search`, `nom flake`, etc. swallows the args and prints its own
# help. Aliases can't express this branch, so a function it is.
#
# `command nix` / `command nom` bypass this function and any shell
# aliases, so we don't recurse.
function nix --wraps nix --description 'nom wrapper: pretty build/develop/shell, passthrough for the rest'
    switch $argv[1]
        case build develop shell
            command nom $argv
        case '*'
            command nix $argv
    end
end
