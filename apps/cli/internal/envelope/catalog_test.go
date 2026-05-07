package envelope

import (
	"reflect"
	"strings"
	"testing"
)

// allCodeConstants is the source-of-truth list of every Code constant
// declared in catalog.go. The test below asserts the catalog map and
// this list are in sync — if a contributor adds a Code constant
// without registering metadata, the build fails here rather than at
// runtime when the unknown code falls back to INTERNAL_ERROR.
//
// Adding a code: declare the const + add an entry to catalog +
// add the const here. All three must change together.
var allCodeConstants = []Code{
	CodeAborted,
	CodeActivationFailed,
	CodeBootstrapRequired,
	CodeBuildFailed,
	CodeConfigInvalid,
	CodeConfigNotFound,
	CodeDeclined,
	CodeInternalError,
	CodeInvalidArgument,
	CodePlanStale,
	CodePreflightFailed,
	CodeStateInvalid,
	CodeStateParseFailed,
	CodeUnknownCommand,
	CodeWorkspaceNotFound,
}

// TestCatalogIsComplete pins that every Code constant has catalog
// metadata. This is the contract Fail() relies on — without it, a
// new error path silently emits with INTERNAL_ERROR's metadata.
func TestCatalogIsComplete(t *testing.T) {
	for _, c := range allCodeConstants {
		if _, ok := Lookup(c); !ok {
			t.Errorf("Code %q has no catalog entry; add to catalog map in catalog.go", c)
		}
	}
	// Reverse direction: the catalog must not carry codes the
	// constants list doesn't know about — otherwise we ship dead
	// metadata for codes nothing emits.
	known := map[Code]bool{}
	for _, c := range allCodeConstants {
		known[c] = true
	}
	for c := range catalog {
		if !known[c] {
			t.Errorf("catalog has metadata for %q but no matching Code constant", c)
		}
	}
}

// TestAllCodesIsSorted pins that AllCodes returns codes in
// lexicographic order. Golden files and docs depend on a stable
// iteration order; without this the docs drift on every build.
func TestAllCodesIsSorted(t *testing.T) {
	got := AllCodes()
	want := make([]Code, len(got))
	copy(want, got)
	for i := 1; i < len(want); i++ {
		for j := i; j > 0 && want[j-1] > want[j]; j-- {
			want[j-1], want[j] = want[j], want[j-1]
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllCodes is not sorted:\ngot:  %v\nwant: %v", got, want)
	}
}

// TestEveryCodeHasNonEmptyFix pins that Fail() will never emit an
// empty fix field by accident. A fix-less error envelope reduces the
// agent's path to "ask the human"; we'd rather force every catalog
// author to write at least the generic remediation.
func TestEveryCodeHasNonEmptyFix(t *testing.T) {
	for _, c := range AllCodes() {
		meta, _ := Lookup(c)
		if strings.TrimSpace(meta.DefaultFix) == "" {
			t.Errorf("Code %q has empty DefaultFix; every catalog entry must carry at least one sentence of remediation", c)
		}
	}
}

// TestCodeIsScreamingSnake pins the naming convention. Mixed-case or
// kebab-case codes would force agents to do case-insensitive matches;
// the SCREAMING_SNAKE convention makes equality checks trivial.
func TestCodeIsScreamingSnake(t *testing.T) {
	for _, c := range AllCodes() {
		s := string(c)
		if s != strings.ToUpper(s) {
			t.Errorf("Code %q must be SCREAMING_SNAKE; mixed case violates the contract", c)
		}
		if strings.ContainsAny(s, "-. ") {
			t.Errorf("Code %q contains non-underscore separator", c)
		}
	}
}
