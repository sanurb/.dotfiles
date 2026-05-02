package stepview

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
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `go test -update` to create)", name, err)
	}
	if string(wantBytes) != got {
		t.Fatalf("golden %s mismatch\n--- want ---\n%s\n--- got ---\n%s", name, string(wantBytes), got)
	}
}

func osRows(cursor, selected, defaultIdx int) []Row {
	labels := []string{"macOS", "linux", "wsl"}
	rows := make([]Row, len(labels))
	for i, l := range labels {
		rows[i] = Row{
			Label:    l,
			Cursor:   i == cursor,
			Selected: i == selected,
			Default:  i == defaultIdx,
		}
	}
	return rows
}

func TestView_Step1_NoBack(t *testing.T) {
	m := Model{
		Steps: 6, Current: 0,
		Title:    "Operating system",
		Subtitle: "Choose your platform.",
		Content:  RenderRows(osRows(0, 0, 0)),
		Keymap:   DefaultKeymap(false),
		Width:    80,
	}
	assertGolden(t, "step1.txt", m.View())
}

func TestView_Step3(t *testing.T) {
	m := Model{
		Steps: 6, Current: 2,
		Title:    "Terminal emulator",
		Subtitle: "Pick the terminal app to configure.",
		Content:  RenderRows(osRows(1, 1, 0)),
		Keymap:   DefaultKeymap(true),
		Width:    80,
	}
	assertGolden(t, "step3.txt", m.View())
}

func TestView_Step6_Confirm(t *testing.T) {
	km := DefaultKeymap(true)
	km.SelectVerb = "confirm"
	m := Model{
		Steps: 6, Current: 5,
		Title:    "Confirm",
		Subtitle: "Review your selections before writing selection.toml.",
		Content:  RenderRows(osRows(0, 0, 0)),
		Keymap:   km,
		Width:    80,
	}
	assertGolden(t, "step6.txt", m.View())
}

func TestView_CursorOnDefault(t *testing.T) {
	m := Model{
		Steps: 6, Current: 0,
		Title:    "Operating system",
		Subtitle: "Choose your platform.",
		Content:  RenderRows(osRows(0, 0, 0)),
		Keymap:   DefaultKeymap(false),
		Width:    80,
	}
	assertGolden(t, "cursor_on_default.txt", m.View())
}

func TestView_CursorOffDefault_SelectionStill(t *testing.T) {
	m := Model{
		Steps: 6, Current: 0,
		Title:    "Operating system",
		Subtitle: "Choose your platform.",
		Content:  RenderRows(osRows(2, 0, 0)),
		Keymap:   DefaultKeymap(false),
		Width:    80,
	}
	assertGolden(t, "cursor_off_default.txt", m.View())
}

func TestView_Narrow80Reflow(t *testing.T) {
	m := Model{
		Steps: 6, Current: 3,
		Title:    "Shell",
		Subtitle: "Choose your login shell.",
		Content:  RenderRows(osRows(2, 0, 0)),
		Keymap:   DefaultKeymap(true),
		Width:    80,
	}
	assertGolden(t, "narrow_80.txt", m.View())

	for _, ln := range strings.Split(normalize(m.View()), "\n") {
		w := 0
		for range ln {
			w++
		}
		if w > 80 {
			t.Fatalf("line exceeds 80 cols (%d): %q", w, ln)
		}
	}
}

// Long list: 12 rows, viewport of 5, cursor at index 8. Verifies the
// "▲ N more above" / "▼ N more below" cues + position counter.
func TestView_LongListScroll(t *testing.T) {
	rows := make([]Row, 12)
	for i := range rows {
		rows[i] = Row{Label: shellName(i), Cursor: i == 8, Selected: i == 2, Default: i == 2}
	}
	v := Viewport{Rows: rows, Cursor: 8, Height: 5}
	body, pos, total := v.Render()

	m := Model{
		Steps: 6, Current: 3,
		Title:     "Shell",
		Subtitle:  "Choose your login shell.",
		Content:   body,
		ListPos:   pos,
		ListTotal: total,
		Keymap:    DefaultKeymap(true),
		Width:     80,
	}
	assertGolden(t, "long_list_scrolled.txt", m.View())
}

// Loading / Empty / Error replace the content slot with an explicit,
// readable state — never a blank screen.
func TestView_LoadingState(t *testing.T) {
	m := Model{
		Steps: 6, Current: 1,
		Title:     "Detecting hardware",
		Subtitle:  "Probing chip and OS version.",
		State:     StateLoading,
		StatusMsg: "Reading sysctl…",
		Keymap:    DefaultKeymap(true),
		Width:     80,
	}
	assertGolden(t, "state_loading.txt", m.View())
}

func TestView_EmptyState(t *testing.T) {
	m := Model{
		Steps: 6, Current: 2,
		Title:     "Terminal emulator",
		Subtitle:  "Pick the terminal app to configure.",
		State:     StateEmpty,
		StatusMsg: "No supported terminals detected on $PATH.",
		Keymap:    DefaultKeymap(true),
		Width:     80,
	}
	assertGolden(t, "state_empty.txt", m.View())
}

func TestView_ErrorState(t *testing.T) {
	m := Model{
		Steps: 6, Current: 4,
		Title:     "Profile",
		Subtitle:  "Materialize the chosen profile.",
		State:     StateError,
		StatusMsg: "Cannot read $HOME/.config: permission denied",
		Keymap:    DefaultKeymap(true),
		Width:     80,
	}
	assertGolden(t, "state_error.txt", m.View())
}

// Help overlay: same Keymap that drives the footer drives the overlay,
// so the keys never disagree across surfaces.
func TestView_HelpOverlay(t *testing.T) {
	m := Model{
		Steps: 6, Current: 2,
		Title:    "Terminal emulator",
		Subtitle: "Pick the terminal app to configure.",
		Content:  RenderRows(osRows(0, 0, 0)),
		Help:     true,
		Keymap:   DefaultKeymap(true),
		Width:    80,
	}
	assertGolden(t, "help_overlay.txt", m.View())
}

func shellName(i int) string {
	names := []string{"sh", "bash", "zsh", "fish", "nu", "elvish", "xonsh", "ion", "dash", "ksh", "tcsh", "oil"}
	return names[i]
}
