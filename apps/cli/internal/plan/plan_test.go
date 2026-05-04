package plan

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlanHashIsStableForSameContent(t *testing.T) {
	p1 := Plan{
		SchemaVersion: SchemaVersion,
		Profile:       "work-mac",
		Host:          Host{Hostname: "alpha", OS: "darwin", Arch: "arm64"},
		Steps: []Step{
			{ID: "s1", Kind: "apply-profile", Action: "~", Summary: "apply", Command: "moon run modules:deploy"},
		},
	}
	p2 := p1
	if p1.ComputeHash() != p2.ComputeHash() {
		t.Fatalf("identical content should produce identical hash; got %s vs %s", p1.ComputeHash(), p2.ComputeHash())
	}
}

func TestPlanHashIgnoresGeneratedAt(t *testing.T) {
	p1 := New("p")
	p1.Steps = []Step{{Kind: "apply-profile", Action: "~", Summary: "apply"}}
	h1 := p1.ComputeHash()

	// Mutate GeneratedAt — hash must not change.
	p2 := p1
	p2.GeneratedAt = p1.GeneratedAt.Add(3600 * 1_000_000_000)
	if p1.ComputeHash() != p2.ComputeHash() {
		t.Errorf("hash must ignore GeneratedAt; before=%s after=%s", h1, p2.ComputeHash())
	}
}

func TestPlanHashChangesWithStepOrder(t *testing.T) {
	p1 := New("p")
	p1.Steps = []Step{
		{Kind: "bootstrap-nix", Action: "+", Summary: "nix"},
		{Kind: "apply-profile", Action: "~", Summary: "apply"},
	}
	p2 := New("p")
	p2.Steps = []Step{
		{Kind: "apply-profile", Action: "~", Summary: "apply"},
		{Kind: "bootstrap-nix", Action: "+", Summary: "nix"},
	}
	if p1.ComputeHash() == p2.ComputeHash() {
		t.Errorf("step order must affect hash")
	}
}

func TestPlanRoundTrip(t *testing.T) {
	p := New("work-mac")
	p.Steps = []Step{
		{ID: "s1", Kind: "apply-profile", Action: "~", Summary: "apply", Command: "moon run modules:deploy"},
	}
	p.Seal()

	var buf bytes.Buffer
	if err := p.Encode(&buf); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := Decode(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Hash != p.Hash {
		t.Errorf("hash mismatch: %s vs %s", got.Hash, p.Hash)
	}
	if got.Profile != p.Profile || len(got.Steps) != 1 {
		t.Errorf("decoded plan does not match: %+v", got)
	}
}

func TestPlanRejectsSchemaMismatch(t *testing.T) {
	bad := strings.NewReader(`{"schemaVersion":99,"hash":"x","profile":"p","host":{},"steps":[]}`)
	_, err := Decode(bad)
	if err == nil || !strings.Contains(err.Error(), "schema mismatch") {
		t.Errorf("expected schema mismatch, got %v", err)
	}
}

func TestPlanCounts(t *testing.T) {
	p := Plan{Steps: []Step{
		{Action: "+"}, {Action: "+"}, {Action: "~"}, {Action: "-"},
	}}
	a, c, r := p.Counts()
	if a != 2 || c != 1 || r != 1 {
		t.Errorf("counts: got (%d,%d,%d)", a, c, r)
	}
}

func TestPlanIsNoOp(t *testing.T) {
	if !(&Plan{}).IsNoOp() {
		t.Errorf("empty plan should be no-op")
	}
	if (&Plan{Steps: []Step{{}}}).IsNoOp() {
		t.Errorf("plan with steps should not be no-op")
	}
}
