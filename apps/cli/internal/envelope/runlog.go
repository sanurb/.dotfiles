package envelope

import (
	"fmt"
	"os"
	"path/filepath"
)

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
func OpenRunLog(runID, name string) (*os.File, string, error) {
	root, err := LogsRoot()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Join(root, runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", path, err)
	}
	return f, path, nil
}
