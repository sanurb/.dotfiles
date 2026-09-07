package agents

import (
	"context"
	"testing"
)

// Every input here is verbatim output captured from the real agent on a
// live host. The regex is loose on purpose; these four spellings are
// why.
func TestSemverMatchesRealVersionOutput(t *testing.T) {
	cases := map[string]string{
		"herdr 0.8.2":            "0.8.2",
		"0.85.1":                 "0.85.1",
		"2.1.263 (Claude Code)":  "2.1.263",
		"codex-cli 0.153.4":      "0.153.4",
		"1.18.25":                "1.18.25",
		"v1.2.3-rc.1":            "1.2.3-rc.1",
		"tool 10.20.30 (build7)": "10.20.30",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			got := semver.FindString(in)
			if got != want {
				t.Errorf("semver.FindString(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestSemverRejectsNonVersions(t *testing.T) {
	for _, in := range []string{"", "unknown", "1.2", "no digits here"} {
		if got := semver.FindString(in); got != "" {
			t.Errorf("semver.FindString(%q) = %q, want no match", in, got)
		}
	}
}

// An agent whose binary is absent reports found=false, not an error: a
// fresh host legitimately has none of these, and `dots agents status`
// must still produce a report there.
func TestObserveAbsentBinary(t *testing.T) {
	a := Agent{Name: "nope", Probe: "dots-agent-that-does-not-exist --version"}
	version, found, err := Observe(context.Background(), a, nil)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if found || version != "" {
		t.Errorf("Observe = (%q, %v), want (\"\", false)", version, found)
	}
}

func TestObserveEmptyProbeIsAnError(t *testing.T) {
	if _, _, err := Observe(context.Background(), Agent{Name: "x"}, nil); err == nil {
		t.Fatal("expected an error for an empty probe")
	}
}

func TestTargetIsSupportedHere(t *testing.T) {
	target, err := Target()
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	switch target {
	case "macos-aarch64", "macos-x86_64", "linux-aarch64", "linux-x86_64":
	default:
		t.Errorf("Target() = %q, outside the BOM's platform vocabulary", target)
	}
}

func TestAssetURL(t *testing.T) {
	a := Agent{
		Name:     "herdr",
		Provider: ProviderGitHubRelease,
		Version:  "0.8.2",
		Repo:     "herdrdev/herdr",
		Asset:    "herdr-{os}-{arch}",
	}
	got, err := AssetURL(a, "macos-aarch64")
	if err != nil {
		t.Fatalf("AssetURL: %v", err)
	}
	// Pinned against the URL herdr.dev's own release manifest publishes
	// for v0.8.2 — the check that our template reproduces upstream.
	const want = "https://github.com/herdrdev/herdr/releases/download/v0.8.2/herdr-macos-aarch64"
	if got != want {
		t.Errorf("AssetURL = %q, want %q", got, want)
	}
}

func TestAssetURLRejectsMalformedTarget(t *testing.T) {
	a := Agent{Provider: ProviderGitHubRelease, Repo: "o/r", Asset: "x-{os}-{arch}", Version: "1.0.0"}
	if _, err := AssetURL(a, "macos"); err == nil {
		t.Fatal("expected an error for a target without an arch")
	}
}

func TestConvergeRefusesNativeProvider(t *testing.T) {
	a := Agent{Name: "claude-code", Provider: ProviderNative, Probe: "claude --version"}
	if err := Converge(context.Background(), a, nil); err == nil {
		t.Fatal("Converge must refuse a native agent")
	}
}

func TestInspectClassifies(t *testing.T) {
	// Probes point at a binary that cannot exist, so every managed row
	// is "missing" and every native row is "unmanaged-missing" — the
	// classification under test, independent of what this host has
	// actually installed.
	m := Manifest{SchemaVersion: SchemaVersion, Agents: []Agent{
		{Name: "managed", Provider: ProviderBun, Version: "1.0.0", Package: "p", Probe: "dots-absent-agent --version"},
		{Name: "native", Provider: ProviderNative, Probe: "dots-absent-agent --version"},
	}}
	got := Inspect(context.Background(), m, nil)
	if len(got) != 2 {
		t.Fatalf("Inspect returned %d rows, want 2", len(got))
	}
	if got[0].State != StateMissing || !got[0].NeedsSync() {
		t.Errorf("managed row = %v (NeedsSync=%v), want missing/true", got[0].State, got[0].NeedsSync())
	}
	if got[1].State != StateUnmanagedMissing || got[1].NeedsSync() {
		t.Errorf("native row = %v (NeedsSync=%v), want unmanaged-missing/false", got[1].State, got[1].NeedsSync())
	}
}
