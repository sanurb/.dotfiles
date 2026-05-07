package state

import (
	"bytes"
	"os"
	"strconv"
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
// from a host that toggled the old `git` capability (now removed) are
// read: parse() seeds from Default(), so the absent `font` key resolves
// to true and the vestigial `git` key is silently ignored. This is the
// shape a host writes if it ran an older `dots install` and has never
// re-installed since the git toggle was retired.
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
		t.Fatalf("parse legacy (must tolerate vestigial git key): %v", err)
	}
	if !got.Capabilities.Font {
		t.Fatalf("legacy state without `font` key: got Font=false, want true (inherits Default())")
	}
}

// TestModulesV1BackcompatAllTrue is the load half of the v1→v2 schema
// migration contract: a v1 file (no [modules] section) parses to
// all-satellites-enabled. A regression here would silently strip
// bat/delta/gh/opencode from existing hosts on the next deploy.
func TestModulesV1BackcompatAllTrue(t *testing.T) {
	v1 := `schema_version = 1

[pillars]
shell       = "fish"
terminal    = "ghostty"
multiplexer = "zellij"

[capabilities]
editor = true
font   = true
`
	got, err := parse(strings.NewReader(v1))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if !got.Modules.Bat || !got.Modules.Delta || !got.Modules.Gh || !got.Modules.Opencode {
		t.Fatalf("v1 backcompat: every satellite should default true, got %+v", got.Modules)
	}
}

// TestModulesRoundTrip pins emit/parse symmetry per satellite. A drift
// between writer and reader would mean a TUI toggle silently doesn't
// reach home.nix, mirroring the protection the Font round-trip test
// provides for capabilities.
func TestModulesRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Modules)
	}{
		{"bat off", func(m *Modules) { m.Bat = false }},
		{"delta off", func(m *Modules) { m.Delta = false }},
		{"gh off", func(m *Modules) { m.Gh = false }},
		{"opencode off", func(m *Modules) { m.Opencode = false }},
		{"all off", func(m *Modules) { *m = Modules{} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Default()
			tc.mutate(&s.Modules)

			var buf bytes.Buffer
			if err := emit(&buf, s); err != nil {
				t.Fatalf("emit: %v", err)
			}
			got, err := parse(&buf)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got.Modules != s.Modules {
				t.Fatalf("Modules round-trip drift: got %+v, want %+v", got.Modules, s.Modules)
			}
		})
	}
}

// TestModulesDefaultIsAllOn pins parity with the home.nix `or true`
// fallbacks. If Default() ever flips a satellite off, the Nix profile's
// behavior diverges between "host with state file" (Default applied)
// and "host without state file" (defaultState applied) — exactly the
// kind of split-brain the schema-version contract exists to prevent.
func TestModulesDefaultIsAllOn(t *testing.T) {
	d := Default().Modules
	if !d.Bat || !d.Delta || !d.Gh || !d.Opencode {
		t.Fatalf("Default().Modules should be all-true, got %+v", d)
	}
}

// TestModulesSkippedReportsOptOuts pins the public accessor used by
// status/profile/doctor surfaces. The slice is order-stable (matches
// emit order) so renderings concatenate predictably.
func TestModulesSkippedReportsOptOuts(t *testing.T) {
	tests := []struct {
		name string
		m    Modules
		want []string
	}{
		{"none skipped", Modules{Bat: true, Delta: true, Gh: true, Opencode: true}, nil},
		{"all skipped", Modules{}, []string{"bat", "delta", "gh", "opencode"}},
		{"partial", Modules{Bat: true, Delta: false, Gh: true, Opencode: false}, []string{"delta", "opencode"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.m.Skipped()
			if len(got) != len(tc.want) {
				t.Fatalf("Skipped() = %v, want %v", got, tc.want)
			}
			for i, name := range tc.want {
				if got[i] != name {
					t.Fatalf("Skipped()[%d] = %q, want %q", i, got[i], name)
				}
			}
		})
	}
}

// TestSaveNormalizesSchemaVersion pins the migration boundary: a v1
// file loaded then saved must emit as the current SchemaVersion. The
// alternative — emit() faithfully writing whatever the loaded file
// claimed — would produce a "v1 file with [modules] section," a
// structural lie about version metadata.
func TestSaveNormalizesSchemaVersion(t *testing.T) {
	v1 := `schema_version = 1

[pillars]
shell       = "fish"
terminal    = "ghostty"
multiplexer = "zellij"

[capabilities]
editor = true
font   = true
`
	parsed, err := parse(strings.NewReader(v1))
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if parsed.SchemaVersion != 1 {
		t.Fatalf("parse should preserve loaded schema_version: got %d, want 1", parsed.SchemaVersion)
	}

	dir := t.TempDir()
	path := dir + "/out.toml"
	if err := Save(path, parsed); err != nil {
		t.Fatalf("Save: %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	wantHeader := "schema_version = " + strconv.Itoa(SchemaVersion) + "\n"
	if !strings.Contains(string(written), wantHeader) {
		t.Fatalf("Save did not normalize schema_version to %d.\nFile:\n%s", SchemaVersion, written)
	}
}

// TestModulesParseRejectsNonBool guards against a hand-edit that
// substitutes the wrong type — same protection capabilities already
// has. A silent ignore would let an opt-out vanish without warning.
func TestModulesParseRejectsNonBool(t *testing.T) {
	bad := `schema_version = 2

[pillars]
shell       = "fish"
terminal    = "ghostty"
multiplexer = "zellij"

[modules]
bat = "yes"
`
	_, err := parse(strings.NewReader(bad))
	if err == nil {
		t.Fatalf("parse should reject non-bool in [modules], got nil")
	}
	if !strings.Contains(err.Error(), "modules.bat") {
		t.Fatalf("error should mention `modules.bat`, got %q", err)
	}
}
