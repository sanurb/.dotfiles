package state

import (
	"bytes"
	"strings"
	"testing"
)

// TestFontRoundTrip pins the wizard → home.nix contract: emit/parse must
// preserve Capabilities.Font across both Yes and No, otherwise the bool
// the user picks in the TUI never reaches the home-manager module.
func TestFontRoundTrip(t *testing.T) {
	for _, want := range []bool{true, false} {
		s := Default()
		s.Capabilities.Font = want

		var buf bytes.Buffer
		if err := emit(&buf, s); err != nil {
			t.Fatalf("emit: %v", err)
		}
		if !strings.Contains(buf.String(), "font") {
			t.Fatalf("emitted TOML missing `font` key:\n%s", buf.String())
		}

		got, err := parse(&buf)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got.Capabilities.Font != want {
			t.Fatalf("Font round-trip: got %v, want %v", got.Capabilities.Font, want)
		}
	}
}

// TestFontDefaultIsTrue documents that "Yes" is the default selection.
// home.nix reads the same default via defaultState in modules/profiles/
// home.nix; if these diverge, a fresh host gets a different answer than
// a host whose state file was just written.
func TestFontDefaultIsTrue(t *testing.T) {
	if !Default().Capabilities.Font {
		t.Fatalf("Default().Capabilities.Font = false, want true")
	}
}

// TestParseLegacyStateInheritsDefault documents how schema-1 state files
// (no `font` key, written before this change) are read: parse() seeds
// from Default(), so the missing key resolves to true. Any change here
// is an implicit migration the docstring on parse() promises.
func TestParseLegacyStateInheritsDefault(t *testing.T) {
	legacy := `schema_version = 1

[pillars]
shell       = "fish"
terminal    = "ghostty"
multiplexer = "zellij"

[capabilities]
editor = true
git    = true
`
	got, err := parse(strings.NewReader(legacy))
	if err != nil {
		t.Fatalf("parse legacy: %v", err)
	}
	if !got.Capabilities.Font {
		t.Fatalf("legacy state without `font` key: got Font=false, want true (inherits Default())")
	}
}
