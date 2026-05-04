package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/sanurb/.dotfiles/apps/cli/internal/dym"
	"github.com/sanurb/.dotfiles/apps/cli/internal/exitcode"
)

const cmdExplainSummary = "Built-in topic browser (plan, generations, exit-codes, xdg, bootstrap)"

// explainTopics holds the markdown blobs shipped with the binary. Topics
// are authored as standalone files under apps/cli/explain/<topic>.md so
// they round-trip through review and are diffable independently of the
// Go that prints them. //go:embed binds them at compile time so the
// binary remains a single static artifact.
//
//go:embed explain/*.md
var explainTopics embed.FS

// runExplain implements `dots explain [<topic>]`. No flags: this verb is
// pure documentation and never mutates anything, so the cliflags.Common
// surface would be noise.
func runExplain(rest []string) int {
	topics, err := listExplainTopics()
	if err != nil {
		fmt.Fprintln(os.Stderr, "explain:", err)
		return exitcode.Failure
	}

	// No topic: list what's available. Stdout, exit 0.
	if len(rest) == 0 {
		printExplainTopicList(os.Stdout, topics)
		return exitcode.Success
	}

	// Extra positional args after the topic are a misuse — the verb
	// is `explain <topic>`, single argument. Catching it here keeps
	// the surface honest rather than silently ignoring.
	if len(rest) > 1 {
		fmt.Fprintln(os.Stderr, "explain: too many arguments; usage: dots explain <topic>")
		return exitcode.Misuse
	}

	requested := rest[0]
	body, err := explainTopics.ReadFile("explain/" + requested + ".md")
	if err == nil {
		// Trim a single trailing newline so we can add exactly one
		// ourselves; embed.FS preserves the file as-authored.
		out := strings.TrimRight(string(body), "\n") + "\n"
		fmt.Fprint(os.Stdout, out)
		return exitcode.Success
	}

	fmt.Fprintf(os.Stderr, "explain: unknown topic %q\n", requested)
	if suggestion, ok := dym.Suggest(requested, topics, 3); ok {
		fmt.Fprintf(os.Stderr, "next: did you mean `dots explain %s`?\n", suggestion)
	} else {
		fmt.Fprintln(os.Stderr, "available topics:")
		for _, t := range topics {
			fmt.Fprintln(os.Stderr, "  "+t)
		}
	}
	return exitcode.Misuse
}

// listExplainTopics enumerates the embedded topic files and returns
// the bare topic names (no extension) in sorted order.
func listExplainTopics() ([]string, error) {
	entries, err := fs.ReadDir(explainTopics, "explain")
	if err != nil {
		return nil, fmt.Errorf("read embedded topics: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".md"))
	}
	sort.Strings(out)
	return out, nil
}

func printExplainTopicList(w *os.File, topics []string) {
	fmt.Fprintln(w, "available topics:")
	for _, t := range topics {
		fmt.Fprintln(w, "  "+t)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "usage: dots explain <topic>")
}
