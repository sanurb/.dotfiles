package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sanurb/.dotfiles/apps/cli/internal/state"
	"github.com/sanurb/.dotfiles/apps/cli/internal/workspace"
)

// Severity for an individual check. Doctor exits non-zero iff any
// required check fails.
type Severity string

const (
	SevPass Severity = "pass"
	SevWarn Severity = "warn"
	SevFail Severity = "fail"
)

type Check struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Severity Severity `json:"severity"`
	Required bool     `json:"required"`
	Pinned   string   `json:"pinned,omitempty"`
	Actual   string   `json:"actual,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

type Report struct {
	Checks []Check `json:"checks"`
	Pass   int     `json:"pass"`
	Warn   int     `json:"warn"`
	Fail   int     `json:"fail"`
}

// Required core binaries — Nix-provided via devenv.nix. Missing any of
// these means the dev shell is not active. `treefmt` is in here because
// the formatting check below depends on it; if the wrapper is missing
// we want a single, clearly-attributed failure rather than a confusing
// "format probe failed" downstream.
//
// `nh` and `nom` are intentionally NOT in this list. nh has its own
// version-gated check (checkNh); nom is optional-but-recommended
// (checkNom) — nh delegates to nom when present and falls back to
// plain nix-build output when absent. Demoting nom to optional is the
// doctor-side expression of the layering decision in modules/moon.yml.
var coreBinaries = []string{"nix", "moon", "proto", "gum", "direnv", "git", "treefmt"}

// nhMinVersion: nh 4.3.0 introduced NH_SHOW_ACTIVATION_LOGS /
// `--show-activation-logs` alongside a breaking change that hides
// home-manager activation output by default. modules/moon.yml's
// deploy task sets the env var so failures stay legible; pinning
// here ensures every dev shell has a binary that honors it.
// First tag shipping `nh home switch <flake>#<name> -- <flags>` was
// v4.0.0 (src/interface.rs:306-307), but the activation-log default
// flipped at v4.3.0, so v4.3.0 is the floor.
// Refs: viperML/nh CHANGELOG.md "4.3.0 → Changed".
const nhMinVersion = "4.3.0"

// Required LSPs — declared in devenv.nix's packages list. Keep in lock
// step: every entry here MUST have a matching nixpkgs attribute. New
// LSPs are added in two places (devenv.nix and here) so the doctor
// fails the deploy if a future contributor edits one without the other.
var requiredLSPs = []string{
	"gopls",
	"rust-analyzer",
	"vtsls",
	"typescript-language-server",
	"vscode-css-language-server",
	"vscode-html-language-server",
	"vscode-json-language-server",
	"lua-language-server",
	"nixd",
}

// Probe command per pinned proto runtime. Anything not listed here is
// reported as "no probe registered" (warn, not fail) so adding a new
// runtime to .prototools doesn't surprise-break the doctor.
var runtimeProbes = map[string][]string{
	"node":   {"node", "--version"},
	"bun":    {"bun", "--version"},
	"go":     {"go", "version"},
	"rust":   {"rustc", "--version"},
	"deno":   {"deno", "--version"},
	"python": {"python", "--version"},
	"moon":   {"moon", "--version"},
}

// Styling — lipgloss is in-process, no exec overhead, and works in
// JSON-suppressed mode (we just don't render). Per SOP, the doctor
// itself must not depend on `gum` because gum is one of the binaries
// being checked — a circular dependency.
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	passStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	failStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	hdrStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
)

// runDoctor is the canonical workspace-health gate, invoked by:
//   - `dots doctor`         (interactive, lipgloss-styled)
//   - `dots doctor --json`  (machine-readable, used in CI/agents)
//   - `moon run cli:check`  (deploy-time gate; exit code is the contract)
func runDoctor(jsonOut bool) int {
	report := Report{}

	report.appendAll(checkCore())
	report.appendAll(checkNh())
	report.appendAll(checkNom())
	report.appendAll(checkRuntimes())
	report.appendAll(checkLSPs())
	report.appendAll(checkFormatting())
	report.appendAll(checkPersona())

	for _, c := range report.Checks {
		switch c.Severity {
		case SevPass:
			report.Pass++
		case SevWarn:
			report.Warn++
		case SevFail:
			report.Fail++
		}
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		report.renderHuman()
	}

	if report.Fail > 0 {
		return 1
	}
	return 0
}

func (r *Report) appendAll(cs []Check) { r.Checks = append(r.Checks, cs...) }

func (r *Report) renderHuman() {
	fmt.Println(titleStyle.Render("dots doctor"))
	fmt.Println(dimStyle.Render(strings.Repeat("─", 60)))

	groups := map[string][]Check{}
	for _, c := range r.Checks {
		groups[c.Category] = append(groups[c.Category], c)
	}
	var cats []string
	for k := range groups {
		cats = append(cats, k)
	}
	sort.Strings(cats)

	for _, cat := range cats {
		fmt.Println()
		fmt.Println(hdrStyle.Render(cat))
		for _, c := range groups[cat] {
			var icon, line string
			switch c.Severity {
			case SevPass:
				icon = passStyle.Render("✓")
			case SevWarn:
				icon = warnStyle.Render("○")
			case SevFail:
				icon = failStyle.Render("✗")
			}
			line = c.Name
			if c.Actual != "" {
				line += dimStyle.Render(" → " + c.Actual)
			}
			if c.Detail != "" {
				line += dimStyle.Render(" (" + c.Detail + ")")
			}
			fmt.Printf("  %s %s\n", icon, line)
		}
	}

	fmt.Println()
	fmt.Println(dimStyle.Render(strings.Repeat("─", 60)))
	summary := fmt.Sprintf("%d pass · %d warn · %d fail", r.Pass, r.Warn, r.Fail)
	if r.Fail > 0 {
		fmt.Println(failStyle.Render(summary))
	} else if r.Warn > 0 {
		fmt.Println(warnStyle.Render(summary))
	} else {
		fmt.Println(passStyle.Render(summary))
	}
}

func checkCore() []Check {
	var out []Check
	for _, bin := range coreBinaries {
		c := Check{Name: bin, Category: "Core (Nix-provided)", Required: true}
		path, err := exec.LookPath(bin)
		if err != nil {
			c.Severity = SevFail
			c.Detail = "not on PATH — run `direnv allow`"
		} else {
			c.Severity = SevPass
			c.Actual = path
		}
		out = append(out, c)
	}
	return out
}

func checkLSPs() []Check {
	var out []Check
	for _, lsp := range requiredLSPs {
		c := Check{Name: lsp, Category: "LSPs (Nix-pinned)", Required: true}
		if _, err := exec.LookPath(lsp); err != nil {
			c.Severity = SevFail
			c.Detail = "missing — declared in devenv.nix; rerun `direnv reload`"
		} else {
			c.Severity = SevPass
		}
		out = append(out, c)
	}
	return out
}

// checkFormatting asserts the workspace is "lint-clean": every Go and
// Nix file already matches the canonical treefmt output. The probe runs
// `treefmt --fail-on-change` from the workspace root — a clean tree
// exits 0; any drift exits non-zero (and incidentally rewrites the
// offending files, which is the desired self-healing behavior on a
// developer machine). Same wrapper, same verdict, as `moon run root:fmt`
// and the pre-commit gate.
func checkFormatting() []Check {
	c := Check{Name: "treefmt", Category: "Formatting (treefmt)", Required: true}
	if _, err := exec.LookPath("treefmt"); err != nil {
		c.Severity = SevFail
		c.Detail = "treefmt missing — declared in devenv.nix; rerun `direnv reload`"
		return []Check{c}
	}
	root, err := workspace.Root()
	if err != nil {
		c.Severity = SevFail
		c.Detail = err.Error()
		return []Check{c}
	}
	cmd := exec.Command("treefmt", "--fail-on-change")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		c.Severity = SevFail
		c.Actual = "drift"
		tail := strings.TrimSpace(string(out))
		if len(tail) > 200 {
			tail = tail[len(tail)-200:]
		}
		c.Detail = "run `moon run root:fmt` — " + tail
		return []Check{c}
	}
	c.Severity = SevPass
	c.Actual = "clean"
	return []Check{c}
}

// checkNh: nh is the lifecycle driver `dots deploy` shells out to.
// Required + version-gated (nhMinVersion). Failure modes that map to
// SevFail: missing binary, --version exec failure, unparseable output,
// version below the floor.
func checkNh() []Check {
	c := Check{
		Name:     "nh",
		Category: "Realization (lifecycle driver)",
		Required: true,
		Pinned:   ">=" + nhMinVersion,
	}
	if _, err := exec.LookPath("nh"); err != nil {
		c.Severity = SevFail
		c.Detail = "missing — declared in devenv.nix; rerun `direnv reload`"
		return []Check{c}
	}
	out, err := exec.Command("nh", "--version").CombinedOutput()
	if err != nil {
		c.Severity = SevFail
		c.Detail = "`nh --version` failed: " + err.Error()
		return []Check{c}
	}
	raw := strings.TrimSpace(string(out))
	c.Actual = raw
	ver := extractSemver(raw)
	if ver == "" {
		c.Severity = SevFail
		c.Detail = "could not parse semver from: " + raw
		return []Check{c}
	}
	if !semverGTE(ver, nhMinVersion) {
		c.Severity = SevFail
		c.Detail = "below floor — deploy requires `--show-activation-logs` semantics"
		return []Check{c}
	}
	c.Severity = SevPass
	return []Check{c}
}

// checkNom: nh delegates build rendering to nom when nom is on $PATH.
// Optional-but-recommended — absence is SevWarn (deploy still works,
// just renders plain `nix build` output). Never SevFail by design;
// this is the doctor-side expression of the layering decision.
func checkNom() []Check {
	c := Check{
		Name:     "nom",
		Category: "Realization (build renderer, optional)",
		Required: false,
	}
	path, err := exec.LookPath("nom")
	if err != nil {
		c.Severity = SevWarn
		c.Detail = "absent — nh will fall back to plain `nix build` output"
		return []Check{c}
	}
	c.Severity = SevPass
	c.Actual = path
	c.Detail = "nh will delegate build rendering to nom"
	return []Check{c}
}

var semverRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

func extractSemver(s string) string {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1] + "." + m[2] + "." + m[3]
}

// semverGTE: strict 3-part numeric comparison. Equal is GTE.
// Pre-release suffixes are not considered (the floor is a release).
func semverGTE(a, b string) bool {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		ai, _ := strconv.Atoi(pa[i])
		bi, _ := strconv.Atoi(pb[i])
		if ai != bi {
			return ai > bi
		}
	}
	return true
}

func checkRuntimes() []Check {
	pins, err := readPrototools()
	if err != nil {
		return []Check{{
			Name: ".prototools", Category: "Runtimes (proto-pinned)",
			Severity: SevFail, Required: true, Detail: err.Error(),
		}}
	}
	var names []string
	for k := range pins {
		names = append(names, k)
	}
	sort.Strings(names)

	var out []Check
	for _, name := range names {
		pin := pins[name]
		c := Check{Name: name, Category: "Runtimes (proto-pinned)", Required: true, Pinned: pin}
		probe, ok := runtimeProbes[name]
		if !ok {
			c.Severity = SevWarn
			c.Detail = "no version probe registered"
			out = append(out, c)
			continue
		}
		bin := probe[0]
		if _, err := exec.LookPath(bin); err != nil {
			c.Severity = SevFail
			c.Detail = bin + " not on PATH"
			out = append(out, c)
			continue
		}
		raw, err := exec.Command(probe[0], probe[1:]...).CombinedOutput()
		if err != nil {
			c.Severity = SevFail
			c.Detail = "version probe failed: " + err.Error()
			out = append(out, c)
			continue
		}
		actual := strings.TrimSpace(string(raw))
		c.Actual = actual
		// Channel-style pins ("stable", "latest", "*") accept any working binary.
		if pin == "stable" || pin == "latest" || pin == "*" {
			c.Severity = SevPass
		} else if strings.Contains(actual, pin) {
			c.Severity = SevPass
		} else {
			c.Severity = SevFail
			c.Detail = "drift: pinned " + pin
		}
		out = append(out, c)
	}
	return out
}

// readPrototools parses the top-level `name = "value"` table of
// .prototools at the workspace root. Sub-tables ([plugins], [settings])
// are ignored — they don't pin runtimes.
func readPrototools() (map[string]string, error) {
	root, err := workspace.Root()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(root, ".prototools"))
	if err != nil {
		return nil, fmt.Errorf("open .prototools: %w", err)
	}
	defer f.Close()

	pinRe := regexp.MustCompile(`^([a-z][a-z0-9_-]*)\s*=\s*"([^"]+)"`)
	out := map[string]string{}
	inSubtable := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inSubtable = true
			continue
		}
		if inSubtable {
			continue
		}
		if m := pinRe.FindStringSubmatch(line); m != nil {
			out[m[1]] = m[2]
		}
	}
	return out, sc.Err()
}

// checkPersona audits the realized environment against the user's
// declared persona in .dots-state.toml. The deploy is the contract;
// this is the verifier that the contract was honored. Three sections:
//
//   - Foundation: atuin / zoxide / starship are mandatory infrastructure.
//     Their absence means modules/home/foundation.nix didn't activate, which
//     is a hard fail.
//
//   - State file: missing/unparseable file warns rather than fails so
//     `dots doctor` remains useful before the first `dots install`.
//
//   - Pillar binaries: the selected shell/terminal/multiplexer must
//     resolve on PATH. Drift here usually means the user edited the
//     state file directly without re-running deploy, or a Home Manager
//     activation regressed.
func checkPersona() []Check {
	const cat = "Persona (state-declared)"
	var out []Check

	// Foundation — non-negotiable, identical across every persona.
	for _, bin := range []string{"atuin", "zoxide", "starship"} {
		c := Check{
			Name: bin, Category: cat, Required: true,
			Detail: "foundation — modules/home/foundation.nix",
		}
		if path, err := exec.LookPath(bin); err != nil {
			c.Severity = SevFail
			c.Detail = "foundation tool missing — `dots deploy` to realize"
		} else {
			c.Severity = SevPass
			c.Actual = path
		}
		out = append(out, c)
	}

	root, err := workspace.Root()
	if err != nil {
		out = append(out, Check{
			Name: ".dots-state.toml", Category: cat,
			Severity: SevWarn, Required: false,
			Detail: "no workspace root: " + err.Error(),
		})
		return out
	}

	s, found, err := state.Load(state.Path(root))
	if err != nil {
		out = append(out, Check{
			Name: ".dots-state.toml", Category: cat,
			Severity: SevFail, Required: true,
			Detail: "unparseable: " + err.Error(),
		})
		return out
	}
	if !found {
		out = append(out, Check{
			Name: ".dots-state.toml", Category: cat,
			Severity: SevWarn, Required: false,
			Detail: "no state file — run `dots install` to capture persona",
		})
		// Without a state file the pillar checks would just probe
		// defaults; that's confusing — bail and let the user run install.
		return out
	}

	if err := s.Validate(); err != nil {
		out = append(out, Check{
			Name: ".dots-state.toml", Category: cat,
			Severity: SevFail, Required: true,
			Detail: err.Error(),
		})
		return out
	}
	out = append(out, Check{
		Name: ".dots-state.toml", Category: cat,
		Severity: SevPass, Required: true,
		Actual: fmt.Sprintf("v%d", s.SchemaVersion),
	})

	// Pillar binaries. The map collapses pillar value → executable name
	// (mostly identity, except nushell→nu and the "none" multiplexer).
	pillarProbes := []struct {
		role  string
		value string
		bin   string // empty = skip (e.g. multiplexer "none")
	}{
		{"shell", s.Pillars.Shell, shellBinary(s.Pillars.Shell)},
		{"terminal", s.Pillars.Terminal, s.Pillars.Terminal},
		{"multiplexer", s.Pillars.Multiplexer, multiplexerBinary(s.Pillars.Multiplexer)},
	}
	for _, p := range pillarProbes {
		c := Check{
			Name: fmt.Sprintf("%s = %s", p.role, p.value), Category: cat, Required: true,
		}
		if p.bin == "" {
			c.Severity = SevPass
			c.Actual = "skipped"
			out = append(out, c)
			continue
		}
		if path, err := exec.LookPath(p.bin); err != nil {
			c.Severity = SevFail
			c.Detail = p.bin + " not on PATH — `dots deploy` to realize persona"
		} else {
			c.Severity = SevPass
			c.Actual = path
		}
		out = append(out, c)
	}
	return out
}

func shellBinary(v string) string {
	if v == "nushell" {
		return "nu"
	}
	return v // fish, zsh
}

func multiplexerBinary(v string) string {
	if v == "none" {
		return ""
	}
	return v // tmux, zellij
}
