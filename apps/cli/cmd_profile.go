package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// Scope note: in this release the verb only inspects the persona
// recorded in `.dots-state.toml`. A future profile catalog will
// extend list/use.
const cmdProfileSummary = "Inspect the active profile (use <name> reserved for a future release)"

// profileShowJSON is the JSON projection of state.State for `profile
// show --json`. Mirrors the field set the human renderer prints; the
// JSON consumer wants the same data plus stable types (booleans, not
// "true"/"false" strings).
type profileShowJSON struct {
	SchemaVersion int    `json:"schemaVersion"`
	Shell         string `json:"shell"`
	Terminal      string `json:"terminal"`
	Multiplexer   string `json:"multiplexer"`
	Editor        bool   `json:"editor"`
	Git           bool   `json:"git"`
	Font          bool   `json:"font"`
}

// runProfile dispatches over the subcommand. We hand-roll the dispatch
// rather than registering nested FlagSets because (a) the surface is
// tiny, and (b) `--json` only applies to `show`; binding it on `list`
// would be misleading.
func runProfile(rest []string) int {
	sub := "list"
	var subRest []string
	if len(rest) > 0 {
		// Allow flags before the subcommand (`dots profile --json show`)
		// by sniffing for the first non-flag positional. This matches
		// the ergonomics most users expect from Cobra-style CLIs even
		// though we do not depend on Cobra.
		idx := firstNonFlagIndex(rest)
		if idx >= 0 {
			sub = rest[idx]
			subRest = append(subRest, rest[:idx]...)
			subRest = append(subRest, rest[idx+1:]...)
		} else {
			subRest = rest
		}
	}

	switch sub {
	case "list":
		return runProfileList(subRest)
	case "show":
		return runProfileShow(subRest)
	case "use":
		return runProfileUse(subRest)
	default:
		fmt.Fprintf(os.Stderr, "profile: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: dots profile [list|show|use <name>]")
		return exitcode.Misuse
	}
}

// firstNonFlagIndex returns the index of the first arg that does not
// look like a flag (does not start with `-`), or -1 if every arg is a
// flag. Used to lift the subcommand out of an args slice that may have
// flags interleaved before it.
func firstNonFlagIndex(args []string) int {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			return i
		}
	}
	return -1
}

// runProfileList prints a single human-readable line summarizing the
// active persona. We do not invent a "profile catalog" we do not have;
// the v1 surface is "your one current selection."
func runProfileList(rest []string) int {
	flagSet := flag.NewFlagSet("profile list", flag.ContinueOnError)
	var common cliflags.Common
	common.Bind(flagSet)
	if code, exit := cliflags.MapParseErr(flagSet.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	root, err := workspace.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile list: workspace required")
		fmt.Fprintln(os.Stderr, "where:", err)
		fmt.Fprintln(os.Stderr, "next: clone the dotfiles repo and run from inside it")
		return exitcode.PreFlight
	}

	s, found, err := state.Load(state.Path(root))
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile list: read state:", err)
		return exitcode.Failure
	}
	if !found {
		fmt.Println("(no profile selected — run `dots install` to capture one)")
		return exitcode.Success
	}
	fmt.Println(s.Pillars.Format() + capabilitiesSummary(s.Capabilities.Editor, s.Capabilities.Git, s.Capabilities.Font))
	return exitcode.Success
}

// runProfileShow pretty-prints every field of the state file. Honors
// --json. Keeps the JSON shape distinct from `dots status --json`
// (which embeds workspace + last-apply) so consumers can target the
// narrower view when that's all they need.
func runProfileShow(rest []string) int {
	flagSet := flag.NewFlagSet("profile show", flag.ContinueOnError)
	var common cliflags.Common
	common.Bind(flagSet)
	if code, exit := cliflags.MapParseErr(flagSet.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	root, err := workspace.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile show: workspace required")
		fmt.Fprintln(os.Stderr, "where:", err)
		fmt.Fprintln(os.Stderr, "next: clone the dotfiles repo and run from inside it")
		return exitcode.PreFlight
	}

	s, found, err := state.Load(state.Path(root))
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile show: read state:", err)
		return exitcode.Failure
	}
	if !found {
		if common.JSON {
			// Emit a typed null so jq consumers can distinguish
			// "no profile" from a parse failure cleanly.
			fmt.Println("null")
			return exitcode.Success
		}
		fmt.Println("(no profile selected — run `dots install` to capture one)")
		return exitcode.Success
	}

	if common.JSON {
		doc := profileShowJSON{
			SchemaVersion: s.SchemaVersion,
			Shell:         s.Pillars.Shell,
			Terminal:      s.Pillars.Terminal,
			Multiplexer:   s.Pillars.Multiplexer,
			Editor:        s.Capabilities.Editor,
			Git:           s.Capabilities.Git,
			Font:          s.Capabilities.Font,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(doc)
		return exitcode.Success
	}

	fmt.Printf("schema_version  %d\n", s.SchemaVersion)
	fmt.Printf("shell           %s\n", s.Pillars.Shell)
	fmt.Printf("terminal        %s\n", s.Pillars.Terminal)
	fmt.Printf("multiplexer     %s\n", s.Pillars.Multiplexer)
	fmt.Printf("editor          %v\n", s.Capabilities.Editor)
	fmt.Printf("git             %v\n", s.Capabilities.Git)
	fmt.Printf("font            %v\n", s.Capabilities.Font)
	return exitcode.Success
}

// runProfileUse is the documented stub. Exit 2 (misuse): the verb is
// reserved but not implemented, and a script that branches on it must
// not treat the call as success.
//
// We write the message to stderr (not stdout) so a caller piping
// `dots profile use foo` into another tool gets a clean empty stdout.
func runProfileUse(_ []string) int {
	fmt.Fprintln(os.Stderr, "profile use: not implemented in this release")
	fmt.Fprintln(os.Stderr, "next: edit .dots-state.toml directly, then run `dots apply`")
	fmt.Fprintln(os.Stderr, "      see `dots explain plan` for the apply contract")
	return exitcode.Misuse
}
