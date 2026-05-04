package rollback

import (
	"errors"
	"reflect"
	"testing"
)

// available builds a lookPath stub that succeeds for any binary in
// the given set and returns os/exec's "not found" otherwise.
func available(bins ...string) func(string) (string, error) {
	set := make(map[string]struct{}, len(bins))
	for _, b := range bins {
		set[b] = struct{}{}
	}
	return func(name string) (string, error) {
		if _, ok := set[name]; ok {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found: " + name)
	}
}

func TestResolveWith(t *testing.T) {
	tests := []struct {
		name       string
		generation string
		present    []string
		wantArgs   []string
		wantErr    error
	}{
		{
			name:     "prefers nh when both present, no generation",
			present:  []string{"nh", "home-manager"},
			wantArgs: []string{"nh", "home", "rollback"},
		},
		{
			name:       "prefers nh when both present, with generation",
			generation: "42",
			present:    []string{"nh", "home-manager"},
			wantArgs:   []string{"nh", "home", "rollback", "42"},
		},
		{
			name:     "falls back to home-manager when nh absent, no generation",
			present:  []string{"home-manager"},
			wantArgs: []string{"home-manager", "--rollback"},
		},
		{
			name:       "falls back to home-manager when nh absent, with generation",
			generation: "7",
			present:    []string{"home-manager"},
			wantArgs:   []string{"home-manager", "--switch-generation", "7"},
		},
		{
			name:    "errors when neither tool present",
			present: []string{},
			wantErr: ErrNoTool,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveWith(tc.generation, available(tc.present...))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(got, tc.wantArgs) {
				t.Fatalf("args = %#v, want %#v", got, tc.wantArgs)
			}
		})
	}
}
