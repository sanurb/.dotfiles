package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// Revalidate the complete selection before moving any files: a saved plan
// or wizard scan may predate a Home Manager activation.
func quarantineConflicts(home, dest string, rels []string) error {
	for _, rel := range rels {
		c, err := collisionAt(home, rel)
		if err != nil {
			return err
		}
		if c == nil {
			return fmt.Errorf("snapshot %s: path is missing or symlink-managed; scan again", rel)
		}
		if _, err := os.Lstat(filepath.Join(dest, rel)); !os.IsNotExist(err) {
			return fmt.Errorf("snapshot %s: backup destination already exists or cannot be checked", rel)
		}
	}
	for _, rel := range rels {
		dst := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("create backup parent: %w", err)
		}
		if err := os.Rename(filepath.Join(home, rel), dst); err != nil {
			return fmt.Errorf("snapshot %s: %w", rel, err)
		}
	}
	return nil
}
