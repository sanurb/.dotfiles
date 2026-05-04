package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// errStopWalk is the sentinel returned from the WalkDir callback once a
// match is recorded; filepath.WalkDir aborts the entire walk on any
// non-nil non-SkipDir error, so we use a private sentinel and discard
// it at the call site rather than relying on SkipDir (which would only
// skip the current directory's remaining siblings).
var errStopWalk = errors.New("why: stop walk")

// The summary deliberately advertises the heuristic limit — `dots why`
// does not evaluate Nix; it greps `.nix` files under modules/ for
// references to the queried path. Confidence is "first match wins."
const cmdWhySummary = "Report which module owns a given path (heuristic)"

// whyResultJSON is the JSON projection of a why result. owner is null
// when nothing matches, so consumers can branch with `.owner == null`
// rather than parsing a sentinel string.
type whyResultJSON struct {
	Path   string `json:"path"`
	Owner  string `json:"owner"` // "modules/home/foo.nix:42" or empty -> null
	Status string `json:"status"`
}

// runWhy implements `dots why <path> [--json]`. The verb is read-only
// and operates over the workspace tree. Without a workspace it cannot
// answer, so it exits PreFlight (4) — the user must clone the repo
// before asking who owns a path.
func runWhy(rest []string) int {
	flagSet := flag.NewFlagSet("why", flag.ContinueOnError)
	var common cliflags.Common
	common.Bind(flagSet)
	if code, exit := cliflags.MapParseErr(flagSet.Parse(rest)); exit {
		return code
	}
	common.Resolve()

	args := flagSet.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "why: path required; usage: dots why <path>")
		return exitcode.Misuse
	}
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "why: too many arguments; usage: dots why <path>")
		return exitcode.Misuse
	}

	root, err := workspace.Root()
	if err != nil {
		fmt.Fprintln(os.Stderr, "why: workspace required to answer ownership questions")
		fmt.Fprintln(os.Stderr, "where:", err)
		fmt.Fprintln(os.Stderr, "next: clone the dotfiles repo and run `dots why` from inside it")
		return exitcode.PreFlight
	}

	target, err := resolveQueryPath(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "why: resolve %q: %v\n", args[0], err)
		return exitcode.Failure
	}

	owner, ferr := findOwner(root, target)
	if ferr != nil {
		fmt.Fprintln(os.Stderr, "why: scan modules:", ferr)
		return exitcode.Failure
	}

	if common.JSON {
		emitWhyJSON(target, owner)
		return exitcode.Success
	}
	if owner == "" {
		fmt.Println("unmanaged: dots does not control this path")
		return exitcode.Success
	}
	fmt.Printf("managed: %s\n", owner)
	return exitcode.Success
}

// emitWhyJSON serializes the result with the same indented JSON style
// every other --json surface uses.
func emitWhyJSON(path, owner string) {
	doc := whyResultJSON{Path: path, Owner: owner, Status: "unmanaged"}
	if owner != "" {
		doc.Status = "managed"
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
}

// resolveQueryPath turns the user-supplied path into an absolute one.
// A leading `~` is expanded; otherwise filepath.Abs handles cwd-relative
// inputs. We do NOT require the path to exist — the user can ask "would
// dots manage this if I created it?" and get a useful answer.
func resolveQueryPath(in string) (string, error) {
	if strings.HasPrefix(in, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home: %w", err)
		}
		if in == "~" {
			return home, nil
		}
		if strings.HasPrefix(in, "~/") {
			return filepath.Join(home, in[2:]), nil
		}
	}
	abs, err := filepath.Abs(in)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// findOwner walks modules/home and modules/profiles for `.nix` files
// and returns the first file:line match referencing the target. Match
// rules, in order of preference (each .nix file is scanned once and the
// first hit, by either rule, is recorded for that file):
//
//  1. Substring match on a $HOME-relative path (e.g. ".config/nvim").
//     This catches `xdg.configFile."nvim/init.lua".source = ...`.
//  2. Substring match on the path's basename. Coarser but useful for
//     references like `programs.fish.enable = true;` when the user
//     queries `~/.config/fish/config.fish`.
//
// We pick the FIRST such file across the module tree (sorted walk for
// determinism). A heuristic with a stable order is more useful than a
// "best" one a script can't predict.
func findOwner(root, target string) (string, error) {
	scanRoots := []string{
		filepath.Join(root, "modules", "home"),
		filepath.Join(root, "modules", "profiles"),
	}

	homeRel := homeRelative(target)
	base := filepath.Base(target)

	// Tokens we will substring-match for. Empty entries are filtered.
	var tokens []string
	if homeRel != "" {
		tokens = append(tokens, homeRel)
	}
	if base != "" && base != "." && base != "/" {
		tokens = append(tokens, base)
	}
	if len(tokens) == 0 {
		return "", nil
	}

	for _, scanRoot := range scanRoots {
		match, err := scanModulesForTokens(scanRoot, tokens)
		if err != nil {
			return "", err
		}
		if match != "" {
			return relPathFor(root, match), nil
		}
	}
	return "", nil
}

// scanModulesForTokens walks scanRoot looking for the first .nix file
// containing any of tokens; returns "<absPath>:<lineNum>" or empty if
// no match. Walks in lexical order via filepath.WalkDir's contract.
// A non-existent scanRoot is not an error — modules/profiles may be
// absent in a partial checkout.
func scanModulesForTokens(scanRoot string, tokens []string) (string, error) {
	if _, err := os.Stat(scanRoot); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var result string
	walkErr := filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".nix") {
			return nil
		}
		hit, scanErr := firstTokenHit(path, tokens)
		if scanErr != nil {
			return scanErr
		}
		if hit > 0 {
			result = fmt.Sprintf("%s:%d", path, hit)
			return errStopWalk
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errStopWalk) {
		return "", walkErr
	}
	return result, nil
}

// firstTokenHit returns the 1-based line number of the first line in
// path that contains any of tokens, or 0 if none match. It uses a
// bufio.Scanner so we don't hold full module files in memory — small
// in practice but the discipline is free.
func firstTokenHit(path string, tokens []string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	// Modules can have very long lines (giant let-bindings). Bump the
	// max token size up from the 64 KiB default to avoid spurious
	// "token too long" errors on real-world Nix files.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		for _, tok := range tokens {
			if strings.Contains(text, tok) {
				return line, nil
			}
		}
	}
	return 0, sc.Err()
}

// homeRelative returns the portion of target that lies under the user's
// home directory, prefixed with `.` when meaningful. For
// `/Users/x/.config/nvim/init.lua` this returns `.config/nvim/init.lua`,
// which is the form that appears verbatim in xdg.configFile expressions.
// Empty string means "not under $HOME"; callers fall back to basename.
func homeRelative(target string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	rel, err := filepath.Rel(home, target)
	if err != nil {
		return ""
	}
	if strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}

// relPathFor projects an absolute file:line into a workspace-relative
// form for human-friendly output. Falls back to the absolute string if
// projection fails.
func relPathFor(root, match string) string {
	idx := strings.LastIndex(match, ":")
	if idx < 0 {
		return match
	}
	abs := match[:idx]
	suffix := match[idx:]
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return match
	}
	return rel + suffix
}
