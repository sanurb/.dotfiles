package main

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestManagedHomePathsTracksGhosttyChildrenAndAllowsParentDirectory(t *testing.T) {
	tests := []struct {
		goos       string
		configPath string
	}{
		{goos: "darwin", configPath: "Library/Application Support/com.mitchellh.ghostty/config.ghostty"},
		{goos: "linux", configPath: ".config/ghostty/config.ghostty"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			paths := managedHomePaths(tt.goos)
			for _, want := range []string{
				tt.configPath,
				".config/ghostty/shaders",
				".config/ghostty/themes",
			} {
				if !slices.ContainsFunc(paths, func(candidate managedHomePath) bool {
					return candidate.rel == want
				}) {
					t.Errorf("managedHomePaths(%q) missing %q", tt.goos, want)
				}
			}
			parentIndex := slices.IndexFunc(paths, func(candidate managedHomePath) bool {
				return candidate.rel == ".config/ghostty"
			})
			if parentIndex < 0 || !paths[parentIndex].allowRealDirectory {
				t.Errorf("managedHomePaths(%q) must allow the real Ghostty parent directory", tt.goos)
			}
		})
	}
}

func TestFindCollisionsReportsUnmanagedGhosttyConfigFile(t *testing.T) {
	home := t.TempDir()
	paths := managedHomePaths(runtime.GOOS)
	configIndex := slices.IndexFunc(paths, func(candidate managedHomePath) bool {
		return strings.HasSuffix(candidate.rel, "/config.ghostty")
	})
	if configIndex < 0 {
		t.Fatal("managedHomePaths() has no Ghostty config path")
	}
	configPath := paths[configIndex].rel
	absolutePath := filepath.Join(home, configPath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolutePath, []byte("unmanaged config\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	collisions, err := findCollisions(home)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(collisions, func(candidate collision) bool {
		return candidate.rel == configPath && candidate.kind == "file"
	}) {
		t.Fatalf("findCollisions() = %+v, want Ghostty config collision %q", collisions, configPath)
	}
}

func TestFindCollisionsReportsGhosttyParentFile(t *testing.T) {
	home := t.TempDir()
	parentPath := filepath.Join(home, ".config/ghostty")
	if err := os.MkdirAll(filepath.Dir(parentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parentPath, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	collisions, err := findCollisions(home)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(collisions, func(candidate collision) bool {
		return candidate.rel == ".config/ghostty" && candidate.kind == "file"
	}) {
		t.Fatalf("findCollisions() = %+v, want Ghostty parent-file collision", collisions)
	}
}
