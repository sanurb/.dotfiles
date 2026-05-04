package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
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

// loadInitialState pre-seeds the wizard's pillar form. The wizard is
// always workspace-backed (ADR-0012) so the lookup is single-source:
// workspace state file or state.Default(). A parse error is non-fatal
// because the wizard is how a user fixes a malformed file.
func loadInitialState(workspaceRoot string) state.State {
	s, found, err := state.Load(state.Path(workspaceRoot))
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: ignoring unreadable workspace state file:", err)
	}
	if found {
		return s
	}
	return state.Default()
}

// newWizardDeps wires the adapters that the install/sync wizard needs.
// All three persist against the workspace root, so missing it is a hard
// stop — but we surface it as an error rather than exiting from a
// constructor so the dispatcher can apply the same workspace-required
// message it uses for deploy/doctor/sync/scan/backup. This keeps a
// single user-facing failure mode for "ran outside the workspace,"
// regardless of which subcommand they invoked.
func newWizardDeps() (ui.Deps, error) {
	root, err := workspace.Root()
	if err != nil {
		return ui.Deps{}, err
	}
	session := &backupSession{}
	return ui.Deps{
		Snapshotter:    fsSnapshotter{session: session},
		StatePersister: tomlPersister{workspaceRoot: root, session: session},
		Initial:        loadInitialState(root),
	}, nil
}
