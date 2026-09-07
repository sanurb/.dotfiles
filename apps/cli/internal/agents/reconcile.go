package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Resolver locates an executable by name. It exists so `dots apply` can
// probe and install against the same PATH it hands `nh` — see
// internal/activation, whose package comment makes reconstructing that
// env anywhere else a regression. A nil Resolver falls back to the
// calling process's PATH, which is what the standalone `dots agents`
// verb wants.
type Resolver func(name string) (string, error)

func (r Resolver) orDefault() Resolver {
	if r == nil {
		return exec.LookPath
	}
	return r
}

// ErrToolchainMissing marks the one convergence failure that is a
// bootstrap ordering artifact rather than a real defect: the package
// manager a provider needs is not on PATH yet. `dots apply` treats it
// as a skip-and-note, matching how install-runtimes handles proto not
// being on PATH during the very apply that installs it. Every other
// failure is real and must not be swallowed.
var ErrToolchainMissing = errors.New("provider toolchain not on PATH")

// State is the verdict for one BOM row on this host.
type State int

const (
	// StateUnknown — the probe itself failed in a way worth surfacing
	// (e.g. the binary answered but printed no parseable version).
	StateUnknown State = iota
	// StateConverged — managed, installed, version matches the BOM.
	StateConverged
	// StateDrifted — managed, installed, version differs from the BOM.
	// This is the state the old activation hooks could never reach:
	// they stopped looking once the binary existed.
	StateDrifted
	// StateMissing — managed and not installed.
	StateMissing
	// StateUnmanaged — native provider, installed. Reported, never
	// converged.
	StateUnmanaged
	// StateUnmanagedMissing — native provider, not installed. dots will
	// not install it; the agent's own installer owns that.
	StateUnmanagedMissing
)

func (s State) String() string {
	switch s {
	case StateConverged:
		return "converged"
	case StateDrifted:
		return "drifted"
	case StateMissing:
		return "missing"
	case StateUnmanaged:
		return "unmanaged"
	case StateUnmanagedMissing:
		return "unmanaged-missing"
	default:
		return "unknown"
	}
}

// Status pairs a BOM row with what this host actually has.
type Status struct {
	Agent    Agent
	Declared string // "" for native agents, which declare no version
	Observed string // "" when not installed
	State    State
	Err      error // non-nil only for StateUnknown
}

// NeedsSync reports whether `dots agents sync` would act on this row.
func (s Status) NeedsSync() bool {
	return s.State == StateDrifted || s.State == StateMissing
}

// Inspect probes every agent in the manifest. Read-only and safe to run
// on any host; it never installs, never writes, and never evaluates
// Nix.
//
// Probes run concurrently because they are not cheap: measured on a
// warm macOS host, the roster costs 1474ms sequentially (pi 533ms,
// opencode 723ms — both are Node programs paying interpreter startup)
// against roughly 700ms when fanned out. Results are written back by
// index, so the output order still matches the manifest and every
// rendering surface stays deterministic.
func Inspect(ctx context.Context, m Manifest, resolve Resolver) []Status {
	out := make([]Status, len(m.Agents))
	var wg sync.WaitGroup
	for i, a := range m.Agents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = inspectOne(ctx, a, resolve)
		}()
	}
	wg.Wait()
	return out
}

func inspectOne(ctx context.Context, a Agent, resolve Resolver) Status {
	s := Status{Agent: a, Declared: a.Version}
	observed, found, err := Observe(ctx, a, resolve)
	if err != nil {
		s.State, s.Err = StateUnknown, err
		return s
	}
	s.Observed = observed
	switch {
	case !a.Managed() && !found:
		s.State = StateUnmanagedMissing
	case !a.Managed():
		s.State = StateUnmanaged
	case !found:
		s.State = StateMissing
	case observed == a.Version:
		s.State = StateConverged
	default:
		s.State = StateDrifted
	}
	return s
}

// InstallDir is where github-release agents land. It matches the
// directory herdr's own installer defaults to (HERDR_INSTALL_DIR) and
// is already on home.sessionPath via modules/home/foundation.nix, so a
// freshly installed binary is on PATH without a re-login.
//
// The directory stays an unmanaged escape hatch as documented in
// modules/home/agent-repos.nix — dots writes individual binaries into
// it and never owns the directory itself.
func InstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// Converge installs the agent at its declared version. Callers must
// have checked Managed() and NeedsSync(); Converge does not re-probe,
// so a caller that skips the check will reinstall unconditionally.
func Converge(ctx context.Context, a Agent, resolve Resolver) error {
	switch a.Provider {
	case ProviderGitHubRelease:
		return convergeGitHubRelease(ctx, a)
	case ProviderBun:
		return convergeBun(ctx, a, resolve)
	case ProviderNative:
		return fmt.Errorf("agents.%s: provider \"native\" is never converged", a.Name)
	default:
		return fmt.Errorf("agents.%s: unknown provider %q", a.Name, a.Provider)
	}
}

// AssetURL renders the download URL for a github-release agent on the
// given target. Exported so `dots agents sync --dry-run` can show the
// exact URL it would fetch without fetching it.
func AssetURL(a Agent, target string) (string, error) {
	asset, err := renderAsset(a.Asset, target)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/releases/download/v%s/%s", releaseHost, a.Repo, a.Version, asset), nil
}

// releaseHost is the release-download origin. A package var rather than
// a constant so the checksum-rejection test can point it at a local
// server: that branch is the one guarantee this provider makes over
// piping an upstream installer into a shell, and it would otherwise be
// untested.
var releaseHost = "https://github.com"

// renderAsset substitutes {os} and {arch} in an asset template from a
// "<os>-<arch>" target key.
func renderAsset(template, target string) (string, error) {
	osName, arch, ok := strings.Cut(target, "-")
	if !ok {
		return "", fmt.Errorf("malformed target %q, want \"<os>-<arch>\"", target)
	}
	r := strings.NewReplacer("{os}", osName, "{arch}", arch)
	return r.Replace(template), nil
}

// downloadTimeout bounds the whole fetch. Agent binaries are tens of
// megabytes; 5 minutes is generous on a slow link and still finite.
const downloadTimeout = 5 * time.Minute

// convergeGitHubRelease downloads the pinned release asset, verifies it
// against the digest recorded in the BOM, and installs it atomically.
//
// The digest check is the reason this provider exists instead of
// shelling out to the upstream installer: herdr's install.sh hardcodes
// its manifest URL to latest.json and derives the version from it, so
// it can only ever install "newest". Fetching the asset ourselves is
// the only way to pin, and once we are fetching it ourselves we own the
// verification the installer would otherwise have done.
func convergeGitHubRelease(ctx context.Context, a Agent) error {
	target, err := Target()
	if err != nil {
		return err
	}
	want, ok := a.SHA256[target]
	if !ok {
		return fmt.Errorf("agents.%s: no sha256 recorded for %s", a.Name, target)
	}
	url, err := AssetURL(a, target)
	if err != nil {
		return err
	}
	dir, err := InstallDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %s", url, resp.Status)
	}

	// Stage beside the destination so the final rename is a same-device
	// operation. A partial download therefore never occupies the real
	// path, and a crash leaves a .tmp file rather than a truncated
	// binary the user would go on to execute.
	tmp, err := os.CreateTemp(dir, "."+a.Name+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath) // no-op once the rename has succeeded
	}()

	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, digest), resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("agents.%s: checksum mismatch for %s\n  want %s (from %s)\n  got  %s",
			a.Name, url, want, RelPath, got)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, a.Name)
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("install %s: %w", dest, err)
	}
	return nil
}

// bunTimeout bounds the global install. Registry resolution plus a
// download of a large agent package; 5 minutes matches downloadTimeout.
const bunTimeout = 5 * time.Minute

// convergeBun installs an exact `pkg@version` through bun's global
// prefix. Unlike the activation hook this replaces, the version is
// explicit: `bun add -g pkg` resolved to whatever was newest at the
// moment a host was first provisioned and then never ran again.
func convergeBun(ctx context.Context, a Agent, resolve Resolver) error {
	bin, err := resolve.orDefault()("bun")
	if err != nil {
		// Wrapped, not formatted: `dots apply` distinguishes this from a
		// real failure so the apply that installs bun does not fail on
		// bun not being there yet.
		return fmt.Errorf("%w: bun (run `proto use`, then re-run `dots agents sync`)", ErrToolchainMissing)
	}
	ctx, cancel := context.WithTimeout(ctx, bunTimeout)
	defer cancel()

	spec := a.Package + "@" + a.Version
	cmd := exec.CommandContext(ctx, bin, "add", "-g", spec)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bun add -g %s: %w\n%s", spec, err, strings.TrimSpace(string(out)))
	}
	return nil
}
