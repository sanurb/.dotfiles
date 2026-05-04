package plan

import "os"

// osHostname is split out so tests can stub it without monkey-patching
// the runtime. The error from os.Hostname is intentionally swallowed
// at the caller — a missing hostname is not a reason to refuse to
// emit a plan; downstream tooling reads the field as best-effort.
func osHostname() (string, error) {
	return os.Hostname()
}
