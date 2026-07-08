package preflight_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/preflight"
)

func TestFrom(t *testing.T) {
	wsErr := errors.New("not in a dotfiles workspace")
	notFound := errors.New("not found")

	tests := []struct {
		name      string
		probes    preflight.Probes
		wantOK    bool
		wantFails []string
	}{
		{
			name:   "all probes pass",
			wantOK: true,
		},
		{
			name:      "workspace missing only",
			probes:    preflight.Probes{Workspace: wsErr},
			wantFails: []string{"workspace: not in a dotfiles workspace"},
		},
		{
			// nh alone is not fatal: apply provisions it via
			// `nix develop -c nh` when nix is present.
			name:   "nh missing but nix present is provisionable",
			probes: preflight.Probes{Nh: notFound},
			wantOK: true,
		},
		{
			name:   "nh and nix both missing fails on both",
			probes: preflight.Probes{Nh: notFound, Nix: notFound},
			wantFails: []string{
				"nh: not on PATH and not at <ws>/.devenv/profile/bin/nh",
				"nix: not on PATH",
			},
		},
		{
			name:      "nix missing only",
			probes:    preflight.Probes{Nix: notFound},
			wantFails: []string{"nix: not on PATH"},
		},
		{
			name:   "all probes fail in declared order",
			probes: preflight.Probes{Workspace: wsErr, Nh: notFound, Nix: notFound},
			wantFails: []string{
				"workspace: not in a dotfiles workspace",
				"nh: not on PATH and not at <ws>/.devenv/profile/bin/nh",
				"nix: not on PATH",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := preflight.From(tc.probes)
			if got.OK() != tc.wantOK {
				t.Fatalf("OK() = %v, want %v (failures=%v)", got.OK(), tc.wantOK, got.Failures)
			}
			if !reflect.DeepEqual(got.Failures, tc.wantFails) {
				t.Fatalf("Failures = %#v, want %#v", got.Failures, tc.wantFails)
			}
		})
	}
}
