package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/cliflags"
)

// stubState lets the table-driven test toggle Validate's outcome
// without instantiating real state.State. The whole point of the
// stateValidator interface in install_headless.go is to let tests do
// exactly this; using state.Default() would couple the install_headless
// contract to whatever the canonical default happens to be.
type stubState struct {
	err error
}

func (s stubState) Validate() error { return s.err }

func TestPlanHeadlessInstall(t *testing.T) {
	invalid := errors.New("invalid")

	tests := []struct {
		name        string
		state       stubState
		common      cliflags.Common
		wantErr     error
		wantRealize bool
		wantArgs    []string
	}{
		{
			name:    "invalid state surfaces as misuse",
			state:   stubState{err: invalid},
			common:  cliflags.Common{NonInteractive: true},
			wantErr: invalid,
		},
		{
			name:        "non-interactive without --yes persists then stops",
			state:       stubState{},
			common:      cliflags.Common{NonInteractive: true},
			wantRealize: false,
		},
		{
			name:        "non-interactive --yes hands off to apply",
			state:       stubState{},
			common:      cliflags.Common{NonInteractive: true, Yes: true},
			wantRealize: true,
			wantArgs:    []string{"apply", "--yes", "--non-interactive"},
		},
		{
			name:        "json/quiet/verbose/profile/dry-run propagate to apply argv",
			state:       stubState{},
			common:      cliflags.Common{NonInteractive: true, Yes: true, JSON: true, Quiet: true, Verbose: 2, Profile: "fish", DryRun: true},
			wantRealize: true,
			wantArgs: []string{
				"apply", "--yes", "--non-interactive",
				"--json", "--quiet", "-v", "-v",
				"--profile", "fish", "--dry-run",
			},
		},
		{
			name:        "no-color forwards even without json",
			state:       stubState{},
			common:      cliflags.Common{NonInteractive: true, Yes: true, NoColor: true},
			wantRealize: true,
			wantArgs:    []string{"apply", "--yes", "--non-interactive", "--no-color"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := planHeadlessInstall(tc.state, tc.common)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if plan.realize != tc.wantRealize {
				t.Fatalf("realize = %v, want %v", plan.realize, tc.wantRealize)
			}
			if !reflect.DeepEqual(plan.applyArgs, tc.wantArgs) {
				t.Fatalf("applyArgs = %v, want %v", plan.applyArgs, tc.wantArgs)
			}
		})
	}
}

// TestApplyArgsFromCommonRejectsNoise pins that we don't smuggle
// flags apply doesn't accept. If apply ever drops one of these from
// its FlagSet, the subprocess would die with "flag provided but not
// defined" and this test would catch the drift before users do.
func TestApplyArgsFromCommonOnlyForwardsApplyFlags(t *testing.T) {
	got := applyArgsFromCommon(cliflags.Common{
		NonInteractive: true, Yes: true,
		JSON: true, NoColor: true, Quiet: true,
		Verbose: 1, Profile: "p", DryRun: true,
	})
	allowed := map[string]struct{}{
		"apply": {}, "--yes": {}, "--non-interactive": {},
		"--json": {}, "--no-color": {}, "--quiet": {},
		"-v": {}, "--profile": {}, "p": {}, "--dry-run": {},
	}
	for _, tok := range got {
		if _, ok := allowed[tok]; !ok {
			t.Fatalf("forwarded unexpected token %q in argv %v", tok, got)
		}
	}
}
