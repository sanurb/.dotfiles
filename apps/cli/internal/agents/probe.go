package agents

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Target is the "<os>-<arch>" key the BOM uses for per-platform asset
// names and digests. The vocabulary is herdr's, because herdr is the
// only github-release agent and matching upstream's spelling keeps the
// BOM copy-pasteable from its release manifest. A second agent with a
// different spelling gets an alias table then, not before.
func Target() (string, error) {
	var os string
	switch runtime.GOOS {
	case "darwin":
		os = "macos"
	case "linux":
		os = "linux"
	default:
		return "", fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	default:
		return "", fmt.Errorf("unsupported architecture %q", runtime.GOARCH)
	}
	return os + "-" + arch, nil
}

// probeTimeout bounds a version probe. An agent that hangs on
// --version (waiting on a lock, a login prompt, or a network call) must
// not wedge `dots agents status`; a timeout reads as "not installed",
// which is the safe verdict — sync will then reinstall it.
const probeTimeout = 5 * time.Second

// semver matches the first dotted-numeric triple in a --version line.
// Deliberately loose, because every agent spells it differently and all
// of these are real, verified outputs:
//
//	herdr 0.8.2
//	0.85.1
//	2.1.263 (Claude Code)
//	codex-cli 0.153.4
//
// Anchoring or demanding a strict semver would break on the next agent
// that adds a prefix. Taking the first match is correct for all four.
var semver = regexp.MustCompile(`\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)

// Observe runs an agent's probe command and extracts the version it
// reports. found=false means the binary is absent or unrunnable, which
// the reconciler treats as "missing" rather than as an error: a fresh
// host legitimately has none of these installed.
//
// The probe string is split on whitespace rather than run through a
// shell. Every probe in the BOM is a bare `<bin> --version`; going
// through a shell would buy nothing and hand the BOM the ability to run
// arbitrary pipelines.
//
// resolve locates the binary; nil means the calling process's PATH. The
// apply path passes a resolver bound to the activation env so probing
// and installing agree on which PATH they are talking about.
func Observe(ctx context.Context, a Agent, resolve Resolver) (version string, found bool, err error) {
	argv := strings.Fields(a.Probe)
	if len(argv) == 0 {
		return "", false, fmt.Errorf("agents.%s: probe is empty", a.Name)
	}
	bin, lerr := resolve.orDefault()(argv[0])
	if lerr != nil {
		return "", false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	out, rerr := exec.CommandContext(ctx, bin, argv[1:]...).Output()
	if rerr != nil {
		// On PATH but not answering: report missing so sync repairs it.
		// Surfacing this as a hard error would make `dots agents status`
		// fail on a half-installed host, which is exactly the host that
		// most needs the report.
		return "", false, nil
	}
	m := semver.Find(out)
	if m == nil {
		return "", true, fmt.Errorf("agents.%s: probe %q printed no version: %q",
			a.Name, a.Probe, strings.TrimSpace(string(out)))
	}
	return string(m), true, nil
}
