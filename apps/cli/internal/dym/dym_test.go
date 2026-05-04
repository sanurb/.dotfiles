package dym

import "testing"

func TestSuggest(t *testing.T) {
	cands := []string{"init", "install", "apply", "deploy", "status", "plan", "diff", "capture", "update", "rollback", "why", "explain", "completion", "profile", "sync", "scan", "backup", "doctor", "version", "help"}

	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"isntall", "install", true},
		{"aplly", "apply", true},
		{"stat", "status", true},
		{"depoly", "deploy", true},
		{"xyzzy", "", false},
		{"completely-different-word", "", false},
		{"completion", "completion", true},
	}
	for _, c := range cases {
		got, ok := Suggest(c.in, cands, 3)
		if got != c.want || ok != c.ok {
			t.Errorf("Suggest(%q) = (%q, %v); want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
