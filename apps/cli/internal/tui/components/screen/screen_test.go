package screen

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func normalize(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	return strings.Join(lines, "\n")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	got = normalize(got)
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `go test -update` to create)", name, err)
	}
	if string(want) != got {
		t.Fatalf("golden %s mismatch\n--- want ---\n%s\n--- got ---\n%s", name, string(want), got)
	}
}

func selectionRows(labels []string, cursor, selected int) []Row {
	rows := make([]Row, len(labels))
	for i, l := range labels {
		rows[i] = Row{
			Label:    l,
			Cursor:   i == cursor,
			Selected: i == selected,
		}
	}
	return rows
}

// TestScreen_Step1_Shell asserts the canonical mid-flow render: stepper
// with completed/current/upcoming, "Step N:" muted heading + accent
// title, muted description, option list with cursor and selection,
// canonical footer.
func TestScreen_Step1_Shell(t *testing.T) {
	steps := []string{"Shell", "Terminal", "Multiplexer", "Editor", "Git", "Confirm"}
	rows := selectionRows([]string{"Fish", "Zsh + Powerlevel10k", "Nushell"}, 0, 0)
	l := Layout{
		Stepper:     &Stepper{Steps: steps, Current: 0},
		Heading:     "Step 1:",
		Title:       "Shell",
		Description: "One shell defines the persona. Atuin/zoxide/starship are baked in regardless.",
		Content:     RenderRows(rows),
		Keymap:      DefaultKeymap(false),
		Width:       80,
	}
	assertGolden(t, "step1_shell.txt", Render(l))
}

func TestScreen_Step6_Confirm_WithAuxiliary(t *testing.T) {
	steps := []string{"Shell", "Terminal", "Multiplexer", "Editor", "Git", "Confirm"}
	rows := selectionRows([]string{"Yes, write profile", "No, cancel"}, 0, 0)
	km := DefaultKeymap(true)
	km.SelectVerb = "confirm"
	km.Mnemonics = []KeymapEntry{{Key: "g", Label: "guide"}}
	l := Layout{
		Stepper:     &Stepper{Steps: steps, Current: 5},
		Heading:     "Step 6:",
		Title:       "Confirm",
		Description: "Review your selections before writing .dots-state.toml.",
		Content:     RenderRows(rows),
		Auxiliary: []AuxRow{
			{Icon: "i", Label: "Persona summary", Key: "i"},
			{Icon: "g", Label: "Open the dots guide", Key: "g"},
		},
		Keymap: km,
		Width:  80,
	}
	assertGolden(t, "step6_confirm_with_aux.txt", Render(l))
}

func TestScreen_LoadingState(t *testing.T) {
	l := Layout{
		Stepper:   &Stepper{Steps: []string{"Scan", "Snapshot", "Realize"}, Current: 0},
		Heading:   "Step 1:",
		Title:     "Scanning",
		State:     StateLoading,
		StatusMsg: "Scanning $HOME for brownfield collisions…",
		Keymap:    Keymap{Quit: "Ctrl-C"},
		Width:     80,
	}
	assertGolden(t, "state_loading.txt", Render(l))
}

func TestScreen_ErrorState(t *testing.T) {
	l := Layout{
		Heading:   "Failure:",
		Title:     "moon run modules:deploy",
		State:     StateError,
		StatusMsg: "exit status 1: nh-home not on $PATH",
		Bordered:  true,
		Keymap:    Keymap{Quit: "Ctrl-C"},
		Width:     80,
	}
	assertGolden(t, "outcome_failed.txt", Render(l))
}

func TestScreen_LongList_Scrolled(t *testing.T) {
	labels := []string{"sh", "bash", "zsh", "fish", "nu", "elvish", "xonsh", "ion", "dash", "ksh", "tcsh", "oil"}
	rows := make([]Row, len(labels))
	for i, lbl := range labels {
		rows[i] = Row{Label: lbl, Cursor: i == 8}
	}
	body, _, _ := Viewport{Rows: rows, Cursor: 8, Height: 5}.Render()
	l := Layout{
		Stepper:     &Stepper{Steps: []string{"Shell", "Terminal"}, Current: 0},
		Heading:     "Step 1:",
		Title:       "Shell",
		Description: "Pick one.",
		Content:     body,
		Keymap:      DefaultKeymap(false),
		Width:       80,
	}
	assertGolden(t, "long_list_scrolled.txt", Render(l))
}

func TestScreen_Footer_KeysAndDescriptions(t *testing.T) {
	km := DefaultKeymap(true)
	km.Quit = "q"
	got := km.Footer()
	want := "↑/k up • ↓/j down • [Enter] select • [Esc] back • [q] quit"
	if normalize(got) != want {
		t.Fatalf("footer mismatch:\n got %q\nwant %q", normalize(got), want)
	}
}

// TestScreen_Stepper_GlyphsAndSeparator verifies the canonical "✓ X →
// ● Y → label-only Z" rendering matches the screenshot.
func TestScreen_Stepper_GlyphsAndSeparator(t *testing.T) {
	l := Layout{Stepper: &Stepper{
		Steps:   []string{"OS", "Terminal", "Font", "Shell", "WM", "Nvim"},
		Current: 5,
	}}
	got := normalize(Render(l))
	want := "✓ OS → ✓ Terminal → ✓ Font → ✓ Shell → ✓ WM → ● Nvim"
	if !strings.HasPrefix(got, want) {
		t.Fatalf("stepper:\n got %q\nwant prefix %q", got, want)
	}
}
