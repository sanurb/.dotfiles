package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// payload is what every server below serves as the release asset.
const payload = "#!/bin/sh\necho 0.0.1\n"

// sha256Hex is computed rather than hardcoded: a literal digest in the
// test would only prove the test agrees with itself.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// serveAsset returns a server that hands out payload for any path, plus
// the release host string to point releaseHost at.
func serveAsset(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// withHome points os.UserHomeDir (and therefore InstallDir) at a temp
// tree so the test never writes into the developer's ~/.local/bin.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func withReleaseHost(t *testing.T, host string) {
	t.Helper()
	prev := releaseHost
	releaseHost = host
	t.Cleanup(func() { releaseHost = prev })
}

// The load-bearing guarantee: a digest that does not match what the BOM
// pins must abort the install and leave nothing behind. Without this,
// the provider is no safer than curl-piped-to-sh.
func TestConvergeRejectsChecksumMismatch(t *testing.T) {
	withReleaseHost(t, serveAsset(t))
	home := withHome(t)

	target, err := Target()
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	a := Agent{
		Name: "fake", Provider: ProviderGitHubRelease, Version: "1.0.0",
		Repo: "o/r", Asset: "fake-{os}-{arch}", Probe: "fake --version",
		SHA256: map[string]string{target: strings.Repeat("0", 64)},
	}
	err = Converge(context.Background(), a, nil)
	if err == nil {
		t.Fatal("Converge accepted a payload whose digest does not match the BOM")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error %q does not name the checksum mismatch", err)
	}

	// Nothing installed, and no staging file left for a later run (or a
	// confused user) to execute.
	dir := filepath.Join(home, ".local", "bin")
	entries, rerr := os.ReadDir(dir)
	if rerr != nil && !os.IsNotExist(rerr) {
		t.Fatalf("read %s: %v", dir, rerr)
	}
	for _, e := range entries {
		t.Errorf("%s left behind after a rejected install", e.Name())
	}
}

func TestConvergeInstallsVerifiedAsset(t *testing.T) {
	withReleaseHost(t, serveAsset(t))
	home := withHome(t)

	target, err := Target()
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	a := Agent{
		Name: "fake", Provider: ProviderGitHubRelease, Version: "1.0.0",
		Repo: "o/r", Asset: "fake-{os}-{arch}", Probe: "fake --version",
		SHA256: map[string]string{target: sha256Hex(payload)},
	}
	if err := Converge(context.Background(), a, nil); err != nil {
		t.Fatalf("Converge: %v", err)
	}

	dest := filepath.Join(home, ".local", "bin", "fake")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat %s: %v", dest, err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read %s: %v", dest, err)
	}
	if string(got) != payload {
		t.Errorf("installed content = %q, want %q", got, payload)
	}
}

func TestConvergeReportsMissingDigestForThisPlatform(t *testing.T) {
	withReleaseHost(t, serveAsset(t))
	withHome(t)
	a := Agent{
		Name: "fake", Provider: ProviderGitHubRelease, Version: "1.0.0",
		Repo: "o/r", Asset: "fake-{os}-{arch}", Probe: "fake --version",
		SHA256: map[string]string{"solaris-sparc": strings.Repeat("a", 64)},
	}
	err := Converge(context.Background(), a, nil)
	if err == nil || !strings.Contains(err.Error(), "no sha256 recorded") {
		t.Fatalf("expected a missing-digest error, got %v", err)
	}
}

func TestConvergeSurfacesHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	withReleaseHost(t, srv.URL)
	withHome(t)

	target, _ := Target()
	a := Agent{
		Name: "fake", Provider: ProviderGitHubRelease, Version: "9.9.9",
		Repo: "o/r", Asset: "fake-{os}-{arch}", Probe: "fake --version",
		SHA256: map[string]string{target: strings.Repeat("a", 64)},
	}
	err := Converge(context.Background(), a, nil)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected the HTTP status in the error, got %v", err)
	}
}
