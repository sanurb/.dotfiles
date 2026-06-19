package state

import (
	"bytes"
	"os"
	"sort"
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

// TestSatellitesManifestNonEmpty guards the embedded SATELLITES file:
// an empty manifest would silently disable every satellite import in
// home.nix without any obvious failure mode.
func TestSatellitesManifestNonEmpty(t *testing.T) {
	if len(Satellites) == 0 {
		t.Fatalf("Satellites is empty; SATELLITES manifest must list at least one module")
	}
	if !sort.StringsAreSorted(Satellites) {
		t.Fatalf("Satellites is not sorted: %v", Satellites)
	}
}

// TestModulesV1BackcompatAllTrue is the load half of the v1→v2 schema
// migration contract: a v1 file (no [modules] section) parses to
// all-satellites-enabled. A regression here would silently strip
// satellites from existing hosts on the next deploy.
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
	for _, name := range Satellites {
		if !got.Modules[name] {
			t.Fatalf("v1 backcompat: satellite %q should default true, got false", name)
		}
	}
}

// TestModulesRoundTrip pins emit/parse symmetry per satellite. A drift
// between writer and reader would mean a TUI toggle silently doesn't
// reach home.nix, mirroring the protection the Font round-trip test
// provides for capabilities. Iterates Satellites so a new entry in
// SATELLITES is exercised automatically.
func TestModulesRoundTrip(t *testing.T) {
	for _, name := range Satellites {
		t.Run(name+" off", func(t *testing.T) {
			s := Default()
			s.Modules[name] = false

			var buf bytes.Buffer
			if err := emit(&buf, s); err != nil {
				t.Fatalf("emit: %v", err)
			}
			got, err := parse(&buf)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got.Modules[name] {
				t.Fatalf("round-trip lost opt-out for %q: got true, want false", name)
			}
			for _, other := range Satellites {
				if other == name {
					continue
				}
				if !got.Modules[other] {
					t.Fatalf("round-trip flipped sibling %q: got false, want true", other)
				}
			}
		})
	}

	t.Run("all off", func(t *testing.T) {
		s := Default()
		for _, name := range Satellites {
			s.Modules[name] = false
		}
		var buf bytes.Buffer
		if err := emit(&buf, s); err != nil {
			t.Fatalf("emit: %v", err)
		}
		got, err := parse(&buf)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		for _, name := range Satellites {
			if got.Modules[name] {
				t.Fatalf("round-trip flipped %q back on after all-off", name)
			}
		}
	})
}

// TestModulesDefaultIsAllOn pins parity with the home.nix `or true`
// fallbacks. If Default() ever flips a satellite off, the Nix profile's
// behavior diverges between "host with state file" (Default applied)
// and "host without state file" (defaultState applied) — exactly the
// kind of split-brain the schema-version contract exists to prevent.
func TestModulesDefaultIsAllOn(t *testing.T) {
	d := Default().Modules
	for _, name := range Satellites {
		if !d[name] {
			t.Fatalf("Default().Modules[%q] = false, want true", name)
		}
	}
}

// TestModulesSkippedReportsOptOuts pins the public accessor used by
// status/profile/doctor surfaces. The slice is order-stable (matches
// emit order) so renderings concatenate predictably.
func TestModulesSkippedReportsOptOuts(t *testing.T) {
	none := Default().Modules
	if got := none.Skipped(); len(got) != 0 {
		t.Fatalf("Skipped() with no opt-outs = %v, want empty", got)
	}

	all := make(Modules, len(Satellites))
	if got := all.Skipped(); len(got) != len(Satellites) {
		t.Fatalf("Skipped() with all-off = %v, want every satellite", got)
	} else {
		for i, name := range Satellites {
			if got[i] != name {
				t.Fatalf("Skipped()[%d] = %q, want %q (manifest order)", i, got[i], name)
			}
		}
	}

	if len(Satellites) >= 2 {
		partial := Default().Modules
		first, last := Satellites[0], Satellites[len(Satellites)-1]
		partial[first] = false
		partial[last] = false
		got := partial.Skipped()
		want := []string{first, last}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("Skipped() partial = %v, want %v (manifest order)", got, want)
		}
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

// TestParseDiscardsUnknownSatellite locks in the typo-tolerance policy:
// a key not in SATELLITES is dropped rather than retained, so a stray
// entry can't get round-tripped into canon by the next Save().
func TestParseDiscardsUnknownSatellite(t *testing.T) {
	in := `schema_version = 2

[pillars]
shell       = "fish"
terminal    = "ghostty"
multiplexer = "zellij"

[modules]
not-a-real-satellite = false
`
	got, err := parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, present := got.Modules["not-a-real-satellite"]; present {
		t.Fatalf("parse retained unknown satellite key; want dropped")
	}
}

// TestEmitListsEverySatellite pins the writer to the manifest. A
// regression where emit() forgets a row would put us back in the
// pre-refactor world where a Save() silently dropped opt-outs that
// the writer didn't know about.
func TestEmitListsEverySatellite(t *testing.T) {
	var buf bytes.Buffer
	if err := emit(&buf, Default()); err != nil {
		t.Fatalf("emit: %v", err)
	}
	out := buf.String()
	for _, name := range Satellites {
		if !strings.Contains(out, name+" ") && !strings.Contains(out, name+"=") {
			t.Fatalf("emit() omitted satellite %q.\nFile:\n%s", name, out)
		}
	}
}
