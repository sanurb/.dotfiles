package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sanurb/.dotfiles/apps/cli/internal/ui"
)

func seedConfig(t *testing.T, root, rel string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("keep my configuration\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertConfigPreserved(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep my configuration\n" {
		t.Fatalf("configuration changed at %s: %q, %v", path, data, err)
	}
}

func TestScanSkipsSymlinkedDirectories(t *testing.T) {
	for _, rel := range []string{".config/ghostty", ".config/nvim", ".config"} {
		t.Run(rel, func(t *testing.T) {
			home, repo := t.TempDir(), t.TempDir()
			seedConfig(t, repo, "config")
			seedConfig(t, repo, "init.lua")
			seedConfig(t, repo, "ghostty/config")
			seedConfig(t, repo, "nvim/init.lua")
			// Match Home Manager's chain through an intermediate store link.
			storeLink := filepath.Join(t.TempDir(), "hm-config")
			if err := os.Symlink(repo, storeLink); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(home, rel)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(storeLink, link); err != nil {
				t.Fatal(err)
			}
			cs, err := findCollisions(home)
			if err != nil || len(cs) != 0 {
				t.Fatalf("managed directory classified as collision: %+v, %v", cs, err)
			}
		})
	}
}

func TestSnapshotEntryPointsPreserveManagedConfig(t *testing.T) {
	for _, entry := range []string{"apply", "wizard", "backup"} {
		t.Run(entry, func(t *testing.T) {
			home, repo := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			original := seedConfig(t, repo, "config")
			if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(repo, filepath.Join(home, ".config/ghostty")); err != nil {
				t.Fatal(err)
			}
			// Old plans refer to the leaf, even though the directory is now
			// managed. Both consumers must reject that stale selection.
			const rel = ".config/ghostty/config"
			switch entry {
			case "apply":
				if err := snapshotConflicts([]string{rel}); err == nil {
					t.Fatal("accepted stale apply plan")
				}
			case "wizard":
				f := fsSnapshotter{session: &backupSession{}}
				if _, err := f.Snapshot([]ui.Collision{{Path: rel, Kind: "file"}}); err == nil {
					t.Fatal("accepted stale wizard scan")
				}
			case "backup":
				if code := runBackup(true); code != 0 {
					t.Fatalf("backup returned %d", code)
				}
			}
			assertConfigPreserved(t, original)
		})
	}
}

func TestSnapshotPreservesWholeBrownfieldDirectory(t *testing.T) {
	home, dest := t.TempDir(), t.TempDir()
	seedConfig(t, home, ".config/ghostty/config")
	seedConfig(t, home, ".config/ghostty/themes/personal")
	seedConfig(t, home, ".config/nvim/init.lua")
	cs, err := findCollisions(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].rel != ".config/ghostty" || cs[1].rel != ".config/nvim" {
		t.Fatalf("expected directory collisions, got %+v", cs)
	}
	if err := quarantineConflicts(home, dest, []string{cs[0].rel, cs[1].rel}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".config/ghostty/config", ".config/ghostty/themes/personal", ".config/nvim/init.lua"} {
		assertConfigPreserved(t, filepath.Join(dest, rel))
		if _, err := os.Lstat(filepath.Join(home, rel)); !os.IsNotExist(err) {
			t.Fatalf("source still exists: %s, %v", rel, err)
		}
	}
}

func TestQuarantineRejectsUnsafeSelectionBeforeMoving(t *testing.T) {
	for _, unsafe := range []string{"../outside", "/absolute", ".", "missing", "managed", "managed/config", "dangling/config"} {
		t.Run(unsafe, func(t *testing.T) {
			home, dest, repo := t.TempDir(), t.TempDir(), t.TempDir()
			original := seedConfig(t, home, ".zshrc")
			seedConfig(t, repo, "config")
			if err := os.Symlink(repo, filepath.Join(home, "managed")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(repo, "absent"), filepath.Join(home, "dangling")); err != nil {
				t.Fatal(err)
			}
			if err := quarantineConflicts(home, dest, []string{".zshrc", unsafe}); err == nil {
				t.Fatal("accepted unsafe selection")
			}
			assertConfigPreserved(t, original)
		})
	}
}

func TestQuarantineDoesNotOverwriteBackup(t *testing.T) {
	home, dest := t.TempDir(), t.TempDir()
	original := seedConfig(t, home, ".zshrc")
	backup := seedConfig(t, dest, ".zshrc")
	if err := quarantineConflicts(home, dest, []string{".zshrc"}); err == nil {
		t.Fatal("accepted existing backup destination")
	}
	assertConfigPreserved(t, original)
	assertConfigPreserved(t, backup)
}
