package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
)

// backupSession is a lazily-created per-run backup directory. The first
// adapter that snapshots calls Dir(); subsequent calls return the same
// path so every snapshot from a single wizard invocation lands together
// under ~/.dots_backups/<ts>/. Without this, a wizard run that both
// rewrites the state file and quarantines colliding $HOME paths would
// scatter its backups across two timestamped dirs.
type backupSession struct {
	once sync.Once
	dir  string
	err  error
}

func (s *backupSession) Dir() (string, error) {
	s.once.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			s.err = err
			return
		}
		s.dir = filepath.Join(home, ".dots_backups", time.Now().Format("20060102_150405"))
		s.err = os.MkdirAll(s.dir, 0o755)
	})
	return s.dir, s.err
}

// fsSnapshotter implements ui.Snapshotter against the real filesystem,
// reusing scan.go's collision detection and backup.go's quarantine logic.
type fsSnapshotter struct {
	session *backupSession
}

func (f fsSnapshotter) Scan() ([]ui.Collision, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cs, err := findCollisions(home)
	if err != nil {
		return nil, err
	}
	out := make([]ui.Collision, len(cs))
	for i, c := range cs {
		out[i] = ui.Collision{Path: c.rel, Kind: c.kind}
	}
	return out, nil
}

func (f fsSnapshotter) Snapshot(cs []ui.Collision) (ui.SnapshotResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return ui.SnapshotResult{}, err
	}
	dest, err := f.session.Dir()
	if err != nil {
		return ui.SnapshotResult{}, fmt.Errorf("snapshot dir: %w", err)
	}
	for _, c := range cs {
		src := filepath.Join(home, c.Path)
		dst := filepath.Join(dest, c.Path)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return ui.SnapshotResult{}, err
		}
		if err := os.Rename(src, dst); err != nil {
			return ui.SnapshotResult{}, fmt.Errorf("snapshot %s: %w", c.Path, err)
		}
	}
	return ui.SnapshotResult{Count: len(cs), Path: dest}, nil
}

// moonRealizer implements ui.Realizer by shelling out to
// `moon run dotfiles:deploy`, capturing stdout/stderr to a buffer so
// the TUI is not corrupted by interleaved deploy logs. On failure, the
// captured output is folded into the error.
//
// DOTS_CAPS is dead. The realizer no longer carries any persona config:
// the .dots-state.toml file at the workspace root is the input, read by
// home.nix via builtins.fromTOML.
type moonRealizer struct{}

func (moonRealizer) Realize() (ui.RealizationResult, error) {
	started := time.Now()
	cmd := exec.Command("moon", "run", "dotfiles:deploy")
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		tail := strings.TrimSpace(stderr.String())
		if len(tail) > 1000 {
			tail = tail[len(tail)-1000:]
		}
		return ui.RealizationResult{}, fmt.Errorf("moon run dotfiles:deploy: %w\n%s", err, tail)
	}
	return ui.RealizationResult{DurationMs: time.Since(started).Milliseconds()}, nil
}

// tomlPersister snapshots the existing .dots-state.toml into the dots
// backup directory before atomically writing the new one. It is the
// only path by which the persona file should ever be mutated — direct
// hand-edits will be overwritten on the next `dots install` without
// the snapshot safety net.
type tomlPersister struct {
	workspaceRoot string
	session       *backupSession
}

func (p tomlPersister) SaveState(s state.State) error {
	path := state.Path(p.workspaceRoot)
	if err := p.snapshotPriorState(path); err != nil {
		return err
	}
	return state.Save(path, s)
}

// snapshotPriorState refuses to overwrite without a recoverable copy.
// A failure here blocks the persist — better than silently stomping
// the prior persona.
func (p tomlPersister) snapshotPriorState(statePath string) error {
	src, err := os.Open(statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open prior state: %w", err)
	}
	defer src.Close()

	sessionDir, err := p.session.Dir()
	if err != nil {
		return fmt.Errorf("snapshot dir: %w", err)
	}
	dest := filepath.Join(sessionDir, "state")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create state snapshot dir: %w", err)
	}
	dst, err := os.Create(filepath.Join(dest, state.FileName))
	if err != nil {
		return fmt.Errorf("create state snapshot: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy state snapshot: %w", err)
	}
	return nil
}

// loadInitialState reads the existing persona at startup so the wizard
// can pre-seed the radio forms. A missing file is not an error: we
// fall through to state.Default() and the user gets the opinionated
// defaults pre-selected.
func loadInitialState(workspaceRoot string) state.State {
	s, _, err := state.Load(state.Path(workspaceRoot))
	if err != nil {
		// Surface a parse error by abandoning the prior file's claims;
		// the wizard will pre-seed defaults and overwrite on save. The
		// alternative — refusing to launch the TUI — is hostile when
		// the user is precisely trying to fix the file via the TUI.
		fmt.Fprintln(os.Stderr, "warning: ignoring unreadable state file:", err)
		return state.Default()
	}
	return s
}

func newWizardDeps() ui.Deps {
	root, err := workspaceRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot locate workspace root:", err)
		os.Exit(1)
	}
	session := &backupSession{}
	return ui.Deps{
		Snapshotter:    fsSnapshotter{session: session},
		Realizer:       moonRealizer{},
		StatePersister: tomlPersister{workspaceRoot: root, session: session},
		Initial:        loadInitialState(root),
	}
}
