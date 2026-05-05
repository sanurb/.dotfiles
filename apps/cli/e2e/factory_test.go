package e2e

import "fmt"

// stateOverrides lets a test specify just the fields it cares about.
// Defaults match a realistic dev profile (fish + ghostty + zellij with
// editor + git + font enabled), per the data-factory rule's "meaningful
// domain data, not dummy values".
type stateOverrides struct {
	Shell       string
	Terminal    string
	Multiplexer string
	Editor      *bool
	Git         *bool
	Font        *bool
}

// buildStateTOML returns the contents of a `.dots-state.toml` ready
// to drop at the workspace root. Defaults match the wizard's typical
// output; overrides let each test mutate fields under test without
// rebuilding the whole TOML by hand.
func buildStateTOML(o stateOverrides) string {
	shell := nonEmpty(o.Shell, "fish")
	terminal := nonEmpty(o.Terminal, "ghostty")
	multiplexer := nonEmpty(o.Multiplexer, "zellij")
	editor := boolOr(o.Editor, true)
	git := boolOr(o.Git, true)
	font := boolOr(o.Font, true)
	return fmt.Sprintf(`schema_version = 1

[pillars]
shell       = %q
terminal    = %q
multiplexer = %q

[capabilities]
editor = %v
git    = %v
font   = %v
`, shell, terminal, multiplexer, editor, git, font)
}

// nixStubBody is the body of a no-op nix stub. Sole purpose: make
// bootstrap.NixPresent return true so computePlan does not insert a
// bootstrap-nix step ahead of apply-profile in tests focused on
// post-bootstrap behavior.
const nixStubBody = "#!/bin/sh\nexit 0\n"

func nonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func boolOr(p *bool, fallback bool) bool {
	if p != nil {
		return *p
	}
	return fallback
}
