// Package activation owns the environment shape `dots apply` hands to
// `nh home switch`. Build is the single seam for env construction;
// same inputs produce the same bytes so every reproduction of an
// activation issue is a unit test. Anywhere else constructing an
// activation env is a regression — direnv-style activation
// (PROTO_HOME, devenv profile bin, etc.) has to be recreated for
// subprocesses that don't inherit it from a real shell, and the
// reproduction cost of getting that wrong is high.
package activation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PATH order matters: the workspace's devenv profile bin must lead
// so `nh` resolves without requiring direnv to have activated the
// shell first.
const envPath = "PATH"

// Build returns parent with three additions:
//   - NH_SHOW_ACTIVATION_LOGS=1 (paired with `--show-activation-logs`
//     in the nh invocation; either alone is sufficient, both is
//     defense in depth against an env-strip layer between dots and nh).
//   - PATH-prepend the workspace's `.devenv/profile/bin` so nh
//     resolves outside an active dev shell.
//   - DOTS_WORKSPACE_ROOT=<workspaceRoot> so home-manager modules can
//     resolve workspace-relative paths (`mkOutOfStoreSymlink` targets,
//     state-file lookups) during the --impure flake eval nh performs.
//
// workspaceRoot may be empty; callers handle "no workspace" by passing
// "" rather than special-casing here. PATH and DOTS_WORKSPACE_ROOT
// only apply when a workspace was resolved.
func Build(parent []string, workspaceRoot string) []string {
	env := setEnvKey(parent, "NH_SHOW_ACTIVATION_LOGS", "1")
	if workspaceRoot != "" {
		env = prependPathEntries(env, []string{
			filepath.Join(workspaceRoot, ".devenv", "profile", "bin"),
		})
		env = setEnvKey(env, "DOTS_WORKSPACE_ROOT", workspaceRoot)
	}
	return env
}

// LookPathIn resolves an executable name against the PATH found in env
// (NOT the calling process's PATH). Necessary because exec.Command
// uses the calling process's PATH at construction; when we hand the
// child a different PATH via cmd.Env, the binary lookup must use that
// PATH explicitly. Returns the absolute path on success.
func LookPathIn(name string, env []string) (string, error) {
	var pathValue string
	prefix := envPath + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			pathValue = strings.TrimPrefix(e, prefix)
			break
		}
	}
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

// setEnvKey returns env with key=val present (replaced or appended).
func setEnvKey(env []string, key, val string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

// prependPathEntries prepends extras (in given order) to env's PATH.
// extras may be nil — env is returned unchanged. When env carries no
// PATH, one is synthesized from extras alone.
func prependPathEntries(env, extras []string) []string {
	if len(extras) == 0 {
		return env
	}
	sep := string(os.PathListSeparator)
	prepend := strings.Join(extras, sep)
	pathPrefix := envPath + "="
	for i, e := range env {
		if strings.HasPrefix(e, pathPrefix) {
			env[i] = pathPrefix + prepend + sep + strings.TrimPrefix(e, pathPrefix)
			return env
		}
	}
	return append(env, pathPrefix+prepend)
}
