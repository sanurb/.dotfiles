package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
)

const cmdCompletionSummary = "Emit shell completion for bash/zsh/fish/nu"

// supportedCompletionShells lists the shells we emit scripts for. The
// no-arg error path prints these so the user gets a precise menu, not
// a vague "see --help".
var supportedCompletionShells = []string{"bash", "zsh", "fish", "nu"}

// runCompletion implements `dots completion <shell>`. Output is the
// completion script on stdout (so users can `dots completion zsh > _dots`
// or eval it directly); the no-arg / unknown-shell paths print a usage
// message to stderr and exit 2 (misuse).
func runCompletion(rest []string) int {
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "completion: shell required")
		fmt.Fprintln(os.Stderr, "supported shells: "+strings.Join(supportedCompletionShells, ", "))
		fmt.Fprintln(os.Stderr, "usage: dots completion <"+strings.Join(supportedCompletionShells, "|")+">")
		return exitcode.Misuse
	}
	if len(rest) > 1 {
		fmt.Fprintln(os.Stderr, "completion: too many arguments; usage: dots completion <shell>")
		return exitcode.Misuse
	}

	shell := rest[0]
	verbs := allKnownNames()
	verbList := strings.Join(verbs, " ")

	switch shell {
	case "bash":
		fmt.Print(bashCompletion(verbList))
	case "zsh":
		fmt.Print(zshCompletion(verbList))
	case "fish":
		fmt.Print(fishCompletion(verbs))
	case "nu", "nushell":
		fmt.Print(nuCompletion(verbs))
	default:
		fmt.Fprintf(os.Stderr, "completion: unknown shell %q\n", shell)
		fmt.Fprintln(os.Stderr, "supported shells: "+strings.Join(supportedCompletionShells, ", "))
		return exitcode.Misuse
	}
	return exitcode.Success
}

// bashCompletion emits a completion function bound to `dots`. Source
// the output (or drop into bash-completion's load path) to enable.
// Subcommand-specific completion is intentionally out of scope; the
// function only completes the first positional argument.
func bashCompletion(verbList string) string {
	return `# bash completion for dots — source this file or place it under
# /etc/bash_completion.d/. First-positional only.
_dots_complete() {
    local cur prev
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "` + verbList + `" -- "$cur") )
    fi
    return 0
}
complete -F _dots_complete dots
`
}

// zshCompletion emits a #compdef-style script. Drop into a directory on
// $fpath named `_dots`, or `eval "$(dots completion zsh)"` for an
// ephemeral session.
func zshCompletion(verbList string) string {
	return `#compdef dots
# zsh completion for dots — place as _dots on $fpath or eval the output
# of "dots completion zsh". First-positional only.
_dots() {
    local -a verbs
    verbs=(` + verbList + `)
    if (( CURRENT == 2 )); then
        compadd -- "${verbs[@]}"
    fi
}
_dots "$@"
`
}

// fishCompletion emits one `complete` directive per verb. fish wants a
// description per completion and quoting them keeps spaces well-defined
// even though we currently use none.
func fishCompletion(verbs []string) string {
	var sb strings.Builder
	sb.WriteString("# fish completion for dots — place under ~/.config/fish/completions/dots.fish\n")
	sb.WriteString("# First-positional only; dots's own help is the source of truth for descriptions.\n")
	for _, v := range verbs {
		sb.WriteString(fmt.Sprintf("complete -c dots -f -n '__fish_use_subcommand' -a %q\n", v))
	}
	return sb.String()
}

// nuCompletion emits a Nushell external completer. Nu's completion
// model is a function that returns a record list; we register it via
// $env.config.completions.external.completer for the `dots` command.
func nuCompletion(verbs []string) string {
	return `# nushell completion for dots — source this file or paste into your config.
# Register the completer:
#   $env.config.completions.external.completer = {|spans|
#       if ($spans.0 == "dots") and (($spans | length) == 2) {
#           ` + "`dots-complete-verbs`" + `
#       } else { null }
#   }
def "dots-complete-verbs" [] {
    [` + nuQuoteList(verbs) + `]
}
`
}

// nuQuoteList renders a Go slice as a nushell list of strings: ["a" "b" ...]
// Nushell uses whitespace as a separator inside list literals.
func nuQuoteList(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%q", x)
	}
	return strings.Join(parts, " ")
}
