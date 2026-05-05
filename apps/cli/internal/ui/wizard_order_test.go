package ui

import (
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
)

// TestWizardOrderShellToConfirm pins the new flow:
//
//	Shell → Terminal → Font → Multiplexer → Editor → Confirm
//
// It asserts both the forward `next` chain and the back-step inverse so
// a future refactor cannot silently move Font without updating Esc too.
func TestWizardOrderShellToConfirm(t *testing.T) {
	expected := []stepID{
		stepShell, stepTerminal, stepFont, stepMultiplexer, stepEditor, stepConfirm,
	}
	for i := 0; i < len(expected)-1; i++ {
		cur, next := expected[i], expected[i+1]
		got := selectionSteps[cur].next
		if got != next {
			t.Fatalf("forward: selectionSteps[%v].next = %v, want %v", cur, got, next)
		}
		if back, ok := previousStep[next]; !ok || back != cur {
			t.Fatalf("back: previousStep[%v] = %v (ok=%v), want %v", next, back, ok, cur)
		}
	}
}

// TestStepperLabelsMatchOrder pins the breadcrumb shown at the top of
// every wizard screen against the stepID order. stepperIndex relies on
// the iota block being contiguous from stepShell..stepConfirm; a change
// to either side without the other yields off-by-one breadcrumbs.
func TestStepperLabelsMatchOrder(t *testing.T) {
	want := []string{"Shell", "Terminal", "Font", "Multiplexer", "Editor", "Confirm"}
	if len(stepperLabels) != len(want) {
		t.Fatalf("stepperLabels length = %d, want %d", len(stepperLabels), len(want))
	}
	for i, label := range want {
		if stepperLabels[i] != label {
			t.Fatalf("stepperLabels[%d] = %q, want %q", i, stepperLabels[i], label)
		}
	}
	if got := stepperIndex(stepFont); got != 2 {
		t.Fatalf("stepperIndex(stepFont) = %d, want 2", got)
	}
	if got := stepperIndex(stepConfirm); got != 5 {
		t.Fatalf("stepperIndex(stepConfirm) = %d, want 5", got)
	}
}

// TestFontStepWritesCapability pins the wizard → state contract for
// stepFont. Cursor 0 = Yes (font on); cursor 1 = No (font off). The
// home-manager module reads `caps.font` and adds the package only when
// true, so a flipped mapping silently produces the wrong artifact.
func TestFontStepWritesCapability(t *testing.T) {
	tests := []struct {
		cursor int
		want   bool
	}{
		{cursor: 0, want: true},
		{cursor: 1, want: false},
	}
	for _, tc := range tests {
		s := state.Default()
		selectionSteps[stepFont].apply(&s, tc.cursor)
		if s.Capabilities.Font != tc.want {
			t.Fatalf("cursor=%d: Capabilities.Font = %v, want %v", tc.cursor, s.Capabilities.Font, tc.want)
		}
	}
}

// TestFontStepCopy pins the title/description shown on stepFont so the
// QA-visible strings stay aligned with the spec.
func TestFontStepCopy(t *testing.T) {
	got := pillarTexts[stepFont]
	if got.title != "Font" {
		t.Fatalf("stepFont title = %q, want %q", got.title, "Font")
	}
	if got.desc != "Iosevka Nerd Font (required for icons)." {
		t.Fatalf("stepFont desc = %q, want %q", got.desc, "Iosevka Nerd Font (required for icons).")
	}
	opts := fontOptions()
	if len(opts) != 2 {
		t.Fatalf("fontOptions: %d rows, want 2", len(opts))
	}
	if opts[0].Label != "Yes, install Iosevka Nerd Font" {
		t.Fatalf("fontOptions[0].Label = %q", opts[0].Label)
	}
	if opts[1].Label != "No, I already have it" {
		t.Fatalf("fontOptions[1].Label = %q", opts[1].Label)
	}
}

// TestGitStepRemoved hard-fails if anyone reintroduces the old git step.
// The test references the non-existent identifier indirectly through
// the maps the wizard uses at runtime; if `stepGit` ever returns, the
// _, ok lookup will succeed and the test will catch it.
func TestGitStepRemoved(t *testing.T) {
	for s := stepShell; s <= stepConfirm; s++ {
		if _, ok := pillarTexts[s]; ok {
			if pillarTexts[s].title == "Git defaults" {
				t.Fatalf("pillarTexts still contains the removed Git step: %v", s)
			}
		}
	}
}
