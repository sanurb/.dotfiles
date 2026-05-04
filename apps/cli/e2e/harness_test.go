package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// dotsBin compiles the dots binary once per `go test` invocation and
// returns its path. Compile cost amortizes across every test in the
// package. Cached errors fail every subsequent test rather than
// silently retrying — a broken build blocks all E2E tests by design.
var (
	binOnce sync.Once
	binPath string
	binErr  error
)

func dotsBin(t *testing.T) string {
	t.Helper()
	binOnce.Do(func() {
		dir, err := os.MkdirTemp("", "dots-e2e-bin-*")
		if err != nil {
			binErr = err
			return
		}
		bin := filepath.Join(dir, "dots")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Dir = ".."
		out, err := cmd.CombinedOutput()
		if err != nil {
			binErr = fmt.Errorf("build dots: %w\n%s", err, out)
			return
		}
		binPath = bin
	})
	if binErr != nil {
		t.Fatal(binErr)
	}
	return binPath
}

// harness is the per-test fixture. Every Harness owns a workspace,
// an isolated home, and a record path the moon stub writes its env
// to. t.TempDir handles cleanup automatically.
type harness struct {
	t          *testing.T
	Workspace  string
	Home       string
	BinDir     string // workspace/_stubs — front of PATH
	MoonRecord string // file the moon stub writes env to
}

// newHarness constructs a fresh fixture and lays down the workspace
// markers (.prototools so workspace.Root resolves; the empty .proto
// tree so layer 2 of resolveMoonCmd can place a moon stub).
func newHarness(t *testing.T) *harness {
	t.Helper()
	// EvalSymlinks resolves macOS's /var → /private/var indirection.
	// dots's workspace.Root() returns the realpath; the harness has
	// to match so assertions compare like-for-like.
	raw := t.TempDir()
	ws, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws, ".prototools"), "# e2e fixture\n")
	mustMkdir(t, filepath.Join(ws, ".proto", "bin"))
	mustMkdir(t, filepath.Join(ws, "_stubs"))
	return &harness{
		t:          t,
		Workspace:  ws,
		Home:       t.TempDir(),
		BinDir:     filepath.Join(ws, "_stubs"),
		MoonRecord: filepath.Join(t.TempDir(), "moon.env"),
	}
}

// withStateFile writes a `.dots-state.toml` so plan/apply resolve a
// profile. The factory in factory_test.go produces realistic content.
func (h *harness) withStateFile(toml string) *harness {
	h.t.Helper()
	mustWrite(h.t, filepath.Join(h.Workspace, ".dots-state.toml"), toml)
	return h
}

// installStub plants a POSIX shell script at the given absolute path
// and marks it executable. Used by both withStub (PATH-front shims)
// and withMoonStub (workspace-resolved moon binary).
func (h *harness) installStub(path, body string) {
	h.t.Helper()
	mustWrite(h.t, path, body)
	mustChmod(h.t, path, 0o755)
}

// withMoonStub plants a POSIX shell script at <ws>/.proto/bin/moon.
// resolveMoonCmd's layer 2 picks it up because the test's PATH
// (BinDir + /usr/bin:/bin) deliberately lacks moon. The stub writes
// its received env to MoonRecord and exits 0.
func (h *harness) withMoonStub() *harness {
	h.installStub(
		filepath.Join(h.Workspace, ".proto", "bin", "moon"),
		"#!/bin/sh\nenv > \"$STUB_MOON_RECORD\"\nexit 0\n",
	)
	return h
}

// withStub installs an arbitrary stub binary on the test's BinDir
// (front of PATH). Used to satisfy bootstrap.NixPresent without a
// real nix install — the stub never actually runs in apply tests.
func (h *harness) withStub(name, body string) *harness {
	h.installStub(filepath.Join(h.BinDir, name), body)
	return h
}

// result is what run returns: captured stdout/stderr and exit code.
type result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// run executes the dots binary against the harness's workspace under
// a deliberately minimal env. The PATH leads with BinDir (test stubs)
// then a bare system PATH — no .proto/bin, no .devenv/profile/bin.
// That is the user's broken shell. It is also what every test below
// asserts dots compensates for.
func (h *harness) run(args ...string) result {
	h.t.Helper()
	cmd := exec.Command(dotsBin(h.t), args...)
	cmd.Dir = h.Workspace
	cmd.Env = []string{
		"PATH=" + h.BinDir + ":/usr/bin:/bin",
		"HOME=" + h.Home,
		"STUB_MOON_RECORD=" + h.MoonRecord,
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		h.t.Fatalf("run dots %v: %v\nstderr: %s", args, err, stderr.String())
	}
	return result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

// moonEnv reads the env dump the moon stub recorded. Returns
// (env, true) when the stub ran, (nil, false) when it didn't —
// the boolean is the binary-truth probe; the map is the contents.
// One accessor over two methods keeps the test surface tight.
func (h *harness) moonEnv() (map[string]string, bool) {
	h.t.Helper()
	body, err := os.ReadFile(h.MoonRecord)
	if err != nil {
		return nil, false
	}
	out := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		if i := strings.IndexByte(line, '='); i > 0 {
			out[line[:i]] = line[i+1:]
		}
	}
	return out, true
}

// mustWrite/mustMkdir/mustChmod are the harness's bottle-feeder
// wrappers around os calls. Test bodies stay flat (no `if err != nil`
// at every step) by routing failures through t.Fatal here.
func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustChmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
