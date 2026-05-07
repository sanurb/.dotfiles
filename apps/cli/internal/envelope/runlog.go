package envelope

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// runLogRetention bounds how many per-run log directories
// OpenRunLog keeps under LogsRoot. ULIDs sort lexicographically by
// time, so pruning the oldest is just dropping the lexically lowest
// names. Set high enough that a debugging session's worth of runs
// is always available; low enough that an unattended host running
// `dots apply` from cron does not grow the tree without bound.
const runLogRetention = 50

// LogsRoot returns the per-run log directory root, honoring
// XDG_STATE_HOME when set and falling back to ~/.dots_logs otherwise.
// The path is the surface every long-running verb references via
// log_path; centralizing it here means a future relocation (e.g.,
// XDG-purist) is a one-line change in one place.
func LogsRoot() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "dots", "logs"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("logs root: user home: %w", err)
	}
	return filepath.Join(home, ".dots_logs"), nil
}

// OpenRunLog creates the per-run log directory and opens the named
// log file inside it. Returns the open file (caller closes) and the
// absolute path that should be reported as log_path on the terminal
// envelope.
//
// The file is opened with O_APPEND so a step that fails after a
// retry does not truncate prior diagnostics; the run-id is unique
// per invocation so cross-run interleaving is not a concern.
//
// Pruning the oldest directories beyond runLogRetention is
// best-effort: a failure to prune does not block opening the new
// log. The cleanup is a side-effect of opening, not a separate
// command, so an unattended host stays bounded without operator
// intervention.
func OpenRunLog(runID, name string) (*os.File, string, error) {
	root, err := LogsRoot()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Join(root, runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log dir %s: %w", dir, err)
	}
	pruneOldRunLogs(root, runLogRetention)
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", path, err)
	}
	return f, path, nil
}

// pruneOldRunLogs removes the oldest per-run directories under root
// when the count exceeds keep. ULIDs sort time-ascending so the
// lexically lowest names are the oldest. Errors are swallowed: this
// is a maintenance side-effect, not a contract surface.
func pruneOldRunLogs(root string, keep int) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	dirs := entries[:0]
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}
	if len(dirs) <= keep {
		return
	}
	names := make([]string, 0, len(dirs))
	for _, d := range dirs {
		names = append(names, d.Name())
	}
	slices.Sort(names)
	for _, name := range names[:len(names)-keep] {
		_ = os.RemoveAll(filepath.Join(root, name))
	}
}
