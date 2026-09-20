package loginshell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const loginShellLauncherDir = ".local/libexec/dots/login-shells"

// prepareResilientLoginShellTarget returns a stable login shell path that can
// fall back to an OS shell while a Nix-backed target is temporarily missing.
func prepareResilientLoginShellTarget(shellName, resolvedTarget string) (string, error) {
	if !loginShellTargetNeedsFallback(resolvedTarget) {
		return resolvedTarget, nil
	}

	home, err := loginShellHomeDirectory()
	if err != nil {
		return "", err
	}
	launcherPath := filepath.Join(home, loginShellLauncherDir, shellName)
	profileTarget := filepath.Join(home, ".nix-profile", "bin", shellName)
	if info, err := os.Stat(profileTarget); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
		resolvedTarget = profileTarget
	}
	if err := writeResilientLoginShellLauncher(launcherPath, resolvedTarget, loginShellFallbackPath()); err != nil {
		return "", err
	}
	return launcherPath, nil
}

// probeResilientLoginShellTarget returns an existing stable launcher without
// creating files; dots doctor uses this read-only variant.
func probeResilientLoginShellTarget(shellName, resolvedTarget string) string {
	if !loginShellTargetNeedsFallback(resolvedTarget) {
		return resolvedTarget
	}
	home, err := loginShellHomeDirectory()
	if err != nil {
		return resolvedTarget
	}
	launcherPath := filepath.Join(home, loginShellLauncherDir, shellName)
	if info, err := os.Stat(launcherPath); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
		return launcherPath
	}
	return resolvedTarget
}

// writeResilientLoginShellLauncher atomically installs a login shell launcher
// whose fallback remains executable when a Nix profile or /nix is unavailable.
func writeResilientLoginShellLauncher(launcherPath, targetPath, fallbackPath string) error {
	if err := os.MkdirAll(filepath.Dir(launcherPath), 0o755); err != nil {
		return fmt.Errorf("login shell launcher: create directory: %w", err)
	}

	body := "#!/bin/sh\n" +
		"target=" + shellSingleQuote(targetPath) + "\n" +
		"fallback=" + shellSingleQuote(fallbackPath) + "\n" +
		"if [ -x \"$target\" ]; then\n" +
		"  exec \"$target\" -l \"$@\"\n" +
		"fi\n" +
		"printf 'dots login shell: %s unavailable; falling back to %s\\n' \"$target\" \"$fallback\" >&2\n" +
		"exec \"$fallback\" -l \"$@\"\n"

	temporary, err := os.CreateTemp(filepath.Dir(launcherPath), ".login-shell-*")
	if err != nil {
		return fmt.Errorf("login shell launcher: create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.WriteString(body); err != nil {
		temporary.Close()
		return fmt.Errorf("login shell launcher: write temporary file: %w", err)
	}
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		return fmt.Errorf("login shell launcher: make executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("login shell launcher: close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, launcherPath); err != nil {
		return fmt.Errorf("login shell launcher: install: %w", err)
	}
	return nil
}

func loginShellTargetNeedsFallback(path string) bool {
	if strings.Contains(path, string(filepath.Separator)+".nix-profile"+string(filepath.Separator)) || strings.HasPrefix(path, "/nix/") {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && strings.HasPrefix(resolved, "/nix/")
}

func loginShellHomeDirectory() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("login shell launcher: resolve home directory: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("login shell launcher: resolve home directory: empty path")
	}
	return home, nil
}

func loginShellFallbackPath() string {
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/sh"
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
