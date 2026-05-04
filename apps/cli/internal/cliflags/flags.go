// Package cliflags is the cross-cutting flag surface every dots
// state-changing verb accepts. Centralizing it keeps the contract
// uniform: --json on `status` and --json on `plan` mean the same
// thing, parse the same way, and interact the same way with --no-color
// and --non-interactive.
//
// Resolution rules (idempotent, applied via Resolve):
//   - NO_COLOR env  ⇒ NoColor
//   - CI       env  ⇒ NonInteractive
//   - --json   flag ⇒ NonInteractive AND NoColor (JSON is for robots;
//     prompts and ANSI would corrupt the parse)
package cliflags

import (
	"errors"
	"flag"
	"os"
	"strconv"

	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
)

// MapParseErr converts a flag.FlagSet's Parse error into a CLI exit code.
// Returns (code, true) when the caller should return code immediately;
// (0, false) when err was nil and the caller should proceed.
//
// The point of the helper is mapping flag.ErrHelp (user typed -h /
// --help) to Success rather than Misuse — letting `dots <verb> --help`
// honor the convention that asking for help is exit 0.
func MapParseErr(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	if errors.Is(err, flag.ErrHelp) {
		return exitcode.Success, true
	}
	return exitcode.Misuse, true
}

// Common is the flag bag every verb registers. Callers Bind it onto
// their FlagSet, parse the args, then Resolve() to apply env defaults
// and the --json overrides.
type Common struct {
	DryRun         bool
	Yes            bool
	NonInteractive bool
	JSON           bool
	Quiet          bool
	NoColor        bool
	Config         string
	Profile        string
	Verbose        int
}

// Bind registers every common flag on fs. Call before fs.Parse(args).
// All long names are listed; short aliases (-y, -q, -v) point to the
// same target so the value lands in the same field regardless.
func (c *Common) Bind(fs *flag.FlagSet) {
	fs.BoolVar(&c.DryRun, "dry-run", false, "compute the plan; perform zero side-effects")
	fs.BoolVar(&c.Yes, "yes", false, "auto-approve every confirmation prompt")
	fs.BoolVar(&c.Yes, "y", false, "auto-approve (alias for --yes)")
	fs.BoolVar(&c.NonInteractive, "non-interactive", false, "refuse to prompt; exit 2 if a prompt is needed")
	fs.BoolVar(&c.JSON, "json", false, "machine-readable output; implies --non-interactive and --no-color")
	fs.BoolVar(&c.Quiet, "quiet", false, "warn-and-up only")
	fs.BoolVar(&c.Quiet, "q", false, "quiet (alias for --quiet)")
	fs.BoolVar(&c.NoColor, "no-color", false, "disable ANSI color (also via NO_COLOR env)")
	fs.StringVar(&c.Config, "config", "", "override config file path")
	fs.StringVar(&c.Profile, "profile", "", "profile name override")
	fs.Var((*verboseLevel)(&c.Verbose), "v", "increase verbosity; repeatable (-v, -v -v, -v -v -v)")
	fs.Var((*verboseLevel)(&c.Verbose), "verbose", "increase verbosity (alias for -v)")
}

// Resolve applies environment defaults and intra-flag overrides.
// Idempotent — call once after Parse.
func (c *Common) Resolve() {
	if os.Getenv("NO_COLOR") != "" {
		c.NoColor = true
	}
	if os.Getenv("CI") != "" {
		c.NonInteractive = true
	}
	if c.JSON {
		c.NonInteractive = true
		c.NoColor = true
	}
}

// verboseLevel is a flag.Value that increments on every Set call so a
// repeated -v stacks. The stdlib flag package treats `-v -v` as two
// invocations of the same flag; this type makes the count observable.
// IsBoolFlag() reports true so `-v` (no argument) parses correctly.
type verboseLevel int

func (v *verboseLevel) String() string   { return strconv.Itoa(int(*v)) }
func (v *verboseLevel) IsBoolFlag() bool { return true }
func (v *verboseLevel) Set(s string) error {
	if s == "true" || s == "" {
		*v++
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*v = verboseLevel(n)
	return nil
}
