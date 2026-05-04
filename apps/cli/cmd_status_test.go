package main

import (
	"errors"
	"testing"
)

func TestClassifyDrift(t *testing.T) {
	tests := []struct {
		name      string
		last      *statusLastApplyJSON
		freshHash string
		freshErr  error
		want      driftKind
	}{
		{
			name:     "fresh plan computation failed → unknown",
			freshErr: errors.New("home unreadable"),
			want:     driftUnknown,
		},
		{
			name:      "no receipt, fresh plan present → noReceipt",
			last:      nil,
			freshHash: "abc",
			want:      driftNoReceipt,
		},
		{
			name:      "receipt with empty hash (post-rollback) → rollback",
			last:      &statusLastApplyJSON{PlanHash: ""},
			freshHash: "abc",
			want:      driftRollback,
		},
		{
			name:      "applied hash equals fresh hash → converged",
			last:      &statusLastApplyJSON{PlanHash: "abc"},
			freshHash: "abc",
			want:      driftConverged,
		},
		{
			name:      "applied hash differs from fresh hash → stale",
			last:      &statusLastApplyJSON{PlanHash: "abc"},
			freshHash: "def",
			want:      driftStale,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyDrift(tc.last, tc.freshHash, tc.freshErr)
			if got != tc.want {
				t.Fatalf("classifyDrift = %v (%q), want %v (%q)", got, got, tc.want, tc.want)
			}
		})
	}
}
