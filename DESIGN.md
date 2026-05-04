---
version: 1
name: dots
description: >-
  Visual contract for the dots TUI installer and any sibling TUI surfaces in
  this monorepo. Rendering target is the terminal (Bubble Tea + Lipgloss + huh).
  Tokens here are the source of truth — wizard steps, doctor output, and any
  future TUI app must consume these tokens and never invent inline values.
target: terminal
renderer: lipgloss
minimum_size:
  columns: 80
  rows: 24

colors:
  surface:
    base:
      hex: "#1a1b26"
      ansi256: 235
      role: assumed terminal background (declared, not painted)
  text:
    primary:
      hex: "#c0caf5"
      ansi256: 189
      role: body copy, default foreground
    muted:
      hex: "#565f89"
      ansi256: 60
      role: secondary copy, captions, command echoes, borders
  accent:
    primary:
      hex: "#7aa2f7"
      ansi256: 111
      role: primary action, current focus, progress fill
    title:
      hex: "#bb9af7"
      ansi256: 141
      role: section titles, wizard step headings
    secondary:
      hex: "#7dcfff"
      ansi256: 117
      role: subtitles, keybind hints, interactive affordances
  status:
    success:
      hex: "#9ece6a"
      ansi256: 149
      role: completed step, OK badge, realize-done
    warn:
      hex: "#e0af68"
      ansi256: 179
      role: degraded state, abort notice, "○" badge
    error:
      hex: "#f7768e"
      ansi256: 210
      role: failure, ✗ badge, deploy gate refusal

typography:
  heading:
    lipgloss: { bold: true, foreground: accent.title }
    margin_bottom: 1
    role: step title (e.g., "dots install"), section header
  subheading:
    lipgloss: { bold: true, foreground: accent.secondary }
    role: secondary section label inside a panel
  body:
    lipgloss: { foreground: text.primary }
    role: prose, default copy
  muted:
    lipgloss: { foreground: text.muted }
    role: explanatory captions, command echo, "Open a new shell…"
  keybind-hint:
    lipgloss: { bold: true, foreground: accent.secondary }
    role: footer key labels (↑↓, enter, q, ?)
  status-success:
    lipgloss: { bold: true, foreground: status.success }
  status-warn:
    lipgloss: { bold: true, foreground: status.warn }
  status-error:
    lipgloss: { bold: true, foreground: status.error }
  command-echo:
    lipgloss: { foreground: text.muted }
    role: shows the actual command being run (e.g., "moon run modules:deploy")

components:
  panel:
    border: rounded
    border_foreground: text.muted
    padding: { rows: 1, columns: 2 }
    margin: { top: 1, bottom: 1 }
    role: |
      Frames OUTCOME surfaces only — done, failed, doctor, aborted.
      Wizard step screens render borderless; the stepper, headings,
      and whitespace carry the structure. Outcome surfaces keep the
      rounded panel to mark terminal states in the flow.

  step-header:
    typography:
      heading_prefix: muted     # "Step N:"
      heading_title:  heading   # focal title
    leading_glyph: none
    trailing_separator: blank line (margin_bottom 1)
    role: |
      The "Step N: <Title>" line beneath the stepper. Prefix is muted
      so the eye lands on the title. Outcome surfaces use the same
      role with prefix glyphs ("✓", "✗") in place of "Step N:".

  stepper:
    layout: inline, separated by " → " in text.muted
    states:
      complete:
        glyph: "✓"
        typography: status-success
        label: status-success
      current:
        glyph: "●"
        typography: { bold: true, foreground: accent.primary }
        label: { bold: true, foreground: accent.primary }
      upcoming:
        glyph: none
        label: muted
    role: |
      Linear progress indicator at the top of every wizard step. One
      stepper per screen; never two trackers on the same view. Welcome
      and outcome surfaces render no stepper. Replaces the previous
      "breadcrumb" component — the rename matches the screen primitive's
      API surface and the linear-flow reference (Gentleman.Dots).

  list-item:
    states:
      cursor:
        prefix: "▸ "
        background: none
        foreground: accent.primary
        bold: true
        role: row the user is currently focused on
      selected:
        prefix: "  "
        foreground: accent.primary
        bold: false
        role: |
          The committed value in a single-select. Carries no [x]; the
          accent foreground is the only signal. The cursor row's accent
          foreground compounds with this state when both are true.
      multiselect-checked:
        prefix: "[x] "
        foreground: accent.primary
        role: chosen in a multi-select (capability matrices, not pillars)
      default-marked:
        prefix: "  "
        suffix: " (default)"
        suffix_typography: muted
        foreground: text.primary
        role: detected/recommended option not yet chosen
      plain:
        prefix: "  "
        foreground: text.primary

  auxiliary-action-row:
    layout: |
      Stack of rows below the option list, preceded by a 24-cell muted
      rule. Each row: icon (one cell + space) + label (body) + optional
      "[<key>]" mnemonic in muted.
    icon: { foreground: accent.secondary }
    label: { typography: body }
    separator: { glyph: "────────────────────────", typography: muted }
    role: |
      Contextual reading material / shortcuts (e.g. "Learn about Neovim",
      "View Keymaps"). Non-focusable in cursor flow; bound to the screen's
      Keymap.Mnemonics and surfaced in the footer alongside the canonical
      key entries. NEVER rendered above the option list.

  keybind-footer:
    layout: inline, separated by " • " in muted
    key_label_format: "[<key>]"        # e.g. [Enter], [Esc], [Ctrl-C]
    arrow_keys_format: "<arrow>/<vim>" # e.g. ↑/k, ↓/j
    key_label: typography keybind-hint
    description: typography muted
    placement: last row of the screen, never wrapped
    role: contract that every screen advertises its exit key

  status-badge:
    states:
      ok:    { glyph: "✓", typography: status-success }
      fail:  { glyph: "✗", typography: status-error }
      warn:  { glyph: "○", typography: status-warn }
    role: terminal-leading mark for outcome rows

  detected-default-marker:
    glyph: "·"
    typography: muted
    placement: trailing, separated from label by a single space
    role: signals "we detected this on your machine" without claiming selection

  spinner-indicator:
    style: { foreground: accent.primary }
    glyph_set: dot
    role: long-running step (scan, snapshot, realize)

  progress-bar:
    gradient: [accent.primary, status.success]
    width: min(60, terminal_width - 10)
    show_percentage: false
    role: estimated-time progress for moon run modules:deploy
---

# DESIGN.md — dots

## Overview

`dots` is an opinionated, Nix-native dotfiles platform — a flake + Home Manager
+ Moon + Proto monorepo whose CLI **declares**, never scripts. The TUI is the
human face of that contract: a wizard that captures persona pillars (shell /
terminal / multiplexer), surfaces brownfield collisions, snapshots them, and
hands realization to `moon run modules:deploy`. Doctor hard-gates deploy.

Visual identity follows from that posture:

- **Calm and decisive.** A single panel at a time. One focal accent. The user
  is never asked to compare two equally weighted colors on the same screen.
- **Honest about what it's doing.** Every long step echoes the underlying
  command in muted text (`moon run modules:deploy`). The TUI is a thin,
  legible shell over the declarative pipeline, not a replacement for it.
- **Latency-budgeted.** Progress is shown when work crosses the
  perceptible-delay threshold (~250 ms tick); short work shows nothing. We do
  not animate decoration.
- **Terminal-native.** No emoji-as-icon, no ASCII shadows, no faked elevation.
  Hierarchy comes from color weight, border, and whitespace.

The user is a developer who picked a Nix-based dotfiles workflow on purpose.
They want fast, reproducible, visible. The tone is collaborator, not concierge.

## Colors

**Included.** Palette is tokyonight, already projected into Ghostty/Zellij to
keep the entire workspace coherent. Tokens are semantic; views must consume
them via `styles.go` constants, never inline `lipgloss.Color`.

**Light-terminal constraint (load-bearing).** This palette assumes a dark
terminal background. Foregrounds like `text.primary` (`#c0caf5`) are
unreadable on a light background, and `styles.go` does not paint a background
to compensate. Two consequences:

1. The CLI's supported environment is dark-themed terminals. This is a
   declared assumption, not a bug. Users on light terminals should switch
   theme before running `dots install`; the doctor step may surface this in
   the future.
2. New tokens MUST NOT be added without a 256-color fallback (recorded in the
   YAML) and a check that the role still reads on the dark surface.

The `surface.base` token is documented but not painted — it describes the
*expected* terminal background, so future widgets that legitimately need a
filled surface (e.g., a selected-row highlight) have a token to anchor to.

Reserved meaning:

- `accent.primary` is the **focal** color: only one element on screen owns it
  at a time (current step OR active selection, never both).
- `status.success / warn / error` are reserved for outcome semantics. They
  must not be repurposed for navigation, branding, or decoration.

## Typography

**Adapted.** Terminal typography is bound to the user's font; we cannot pick
families, weights beyond bold/faint, or sizes. Instead, this section defines
**semantic roles** that map to Lipgloss styling primitives (bold, italic,
faint, foreground). All roles are listed in the YAML `typography` block.

Rules:

- `heading` is the only role that sets `margin_bottom: 1`. Other roles are
  inline and add no vertical space.
- Italic is intentionally unused. Many terminal fonts render italic as a
  fallback that looks worse than upright; we don't depend on it.
- `command-echo` is a distinct role from `muted` even though they currently
  share the same Lipgloss style — separating them now lets a future change
  (e.g., a dim monospace marker) apply to commands without rippling.

## Layout

**Adapted.** Spacing is measured in **terminal cells**, never pixels. Each
column is one character, each row is one line. The YAML `minimum_size`
declares the supported floor: **80 columns × 24 rows**. Below that, the TUI
still runs but is allowed to clip; we do not design for sub-80-column phones.

Conventions:

- Wizard step screens render **borderless** — the stepper, headings, and
  whitespace carry the structure. A rounded `panel` is reserved for
  outcome surfaces (done, failed, doctor, aborted) where the frame marks
  a terminal state in the flow.
- The canonical screen template is the slot order owned by
  `components/screen.Layout`: stepper, blank row, "Step N: <Title>" header,
  description, blank row, content (option list or status), optional
  auxiliary action panel preceded by a muted rule, blank row, footer.
  Views never reorder these slots.
- One column of attention, top to bottom. No side-by-side panels.
- Reflow: the only width-responsive element is the `progress-bar`, which is
  `min(60, terminal_width - 10)`.
- Vertical density: between logical sections, insert exactly one blank
  row. Never two; never zero.

## Elevation & Depth

**Omitted with rationale.** Terminals are flat — there is no z-axis, no
shadow, no blur. Hierarchy in `dots` is conveyed by three flat mechanisms,
in this priority order:

1. **Color weight.** `accent.title` > `text.primary` > `text.muted`. The eye
   lands on the title because it is the only saturated magenta on screen.
2. **Border.** The rounded `panel` border marks **outcome surfaces** —
   done, failed, doctor, aborted. It is the equivalent of a "card" in a
   GUI design system and the only depth cue we use; wizard steps in
   progress are intentionally borderless so the user perceives the flow,
   not a sequence of framed dialogs.
3. **Whitespace.** A blank row between blocks does the work that elevation
   would do in a GUI.

We do not draw nested borders, indented sub-panels, or ASCII drop shadows.
If a future surface seems to need them, the answer is to split the flow into
a separate step, not to fake depth.

## Shapes

**Omitted with rationale.** Terminals have no corner radii or stroke widths.
The only shape concern is *which* box-drawing character set frames the
panel, and that is captured under `components.panel.border: rounded` —
i.e., the Lipgloss `RoundedBorder()` glyphs. Treating that as a Shapes-level
token would imply we have multiple shape primitives; we don't, and we
shouldn't.

## Components

**Included.** Every component has tokens in the YAML frontmatter. Below is
the prose contract for each. Views must compose these atoms; they must not
introduce new ones without first adding tokens here.

- **`panel`** — outcome surfaces only (done, failed, doctor, aborted).
  Rounded border in `text.muted`, padding 1×2, margin 1 top / 1 bottom.
  Wizard steps in progress are borderless.

- **`step-header`** — `Step N:` muted prefix + focal title in
  `accent.title`. Examples: `Step 6: Confirm`, `✓ Realized` (where the
  leading glyph stands in for the prefix on outcome surfaces). No leading
  glyph in the wizard flow itself.

- **`stepper`** — three states (`complete`, `current`, `upcoming`).
  Inline, separated by ` → ` in `muted`. Complete steps render `✓ Label`
  in `status.success`; the current step renders `● Label` in
  `accent.primary` bold; upcoming steps render the label only, in
  `muted`. This is the **only** place where all three status/accent
  colors coexist, because each is confined to a different stepper cell.

- **`list-item`** — five states (`cursor`, `selected`, `multiselect-checked`,
  `default-marked`, `plain`). The `cursor` row is the focus indicator: `▸`
  prefix in `accent.primary`, bold; the row's text adopts `accent.primary`
  too. `selected` is the committed value in a single-select — no `[x]`,
  just `accent.primary` foreground. `multiselect-checked` keeps `[x] ` in
  `accent.primary` for capability matrices. `default-marked` carries a
  trailing ` (default)` in `muted`. `plain` is two leading spaces and
  `text.primary`. A row may be `cursor` and `selected` simultaneously; in
  that case the `▸` prefix wins on the lead and `accent.primary` is the
  text foreground throughout.

- **`auxiliary-action-row`** — the contextual affordance block below the
  option list. Preceded by a 24-cell muted rule, then one row per
  affordance: icon (one cell + space) in `accent.secondary`, label in
  `body`, optional `[<key>]` mnemonic in `muted`. Bound to the screen's
  Keymap.Mnemonics so the same keys advertised in the footer drive the
  rows. The block sits **below** the option list — never above it — so
  the cursor flow (focus → commit) reaches the option list first.

- **`keybind-footer`** — last row of the screen, never wrapped. Entries
  separated by ` • ` in `muted`. Key labels render as `[<key>]` in
  `keybind-hint` (arrow keys keep the `↑/k`, `↓/j` form); each followed
  by a `muted` description. Every screen MUST advertise its exit key.

- **`status-badge`** — three states (`ok`, `fail`, `warn`) using the
  glyphs `✓ / ✗ / ○`. Used as a row leader on outcome lines (the "done"
  summary, the doctor table). Never used inline inside body prose — body
  prose uses words, not badges.

- **`detected-default-marker`** — single `·` in `muted`, trailing the
  label. Used in select lists when a value was auto-detected from the
  environment. Distinct from `default-marked` list-item state, which marks
  the item the wizard would pick if the user hits enter; the dot marks
  *what was found on the machine*. Both can appear on the same row.

- **`spinner-indicator`** / **`progress-bar`** — shown only when work
  crosses the perceptible-delay threshold (~250 ms). Spinner uses the dot
  set in `accent.primary`. Progress bar gradients from `accent.primary` to
  `status.success` and hides percentage; the muted command echo
  underneath communicates ground truth.

## Do's and Don'ts

**Do reserve `accent.primary` for one element at a time.** The current step
*or* the user's active selection — never both at once. If two things on
screen claim primacy, neither feels primary.

**Do echo the underlying command in `command-echo`** during long steps.
The TUI is a face on a declarative pipeline; hiding `moon run
modules:deploy` makes the system feel magical and untrustable.

**Do degrade gracefully under `NO_COLOR` and 8-color terminals.** Every
hex token has a `ansi256` fallback in the YAML; if a renderer drops to
8-color, semantic roles must still be distinguishable (success ≠ error)
even if shades flatten.

**Do collapse decoration when the work is fast.** No spinner for sub-250 ms
operations. No progress bar for instant ones. Stillness is honest.

**Don't introduce new colors in views.** If a screen wants a color, the
answer is either an existing token or a new one added to this file —
never an inline `lipgloss.Color` literal.

**Don't repurpose `status.success / warn / error` for navigation or
branding.** Green means "this finished OK." If you use it for "next
step," you've broken the user's only outcome signal.

**Don't fake elevation.** No ASCII drop shadows, no nested borders, no
double-line frames. Use whitespace and color weight.

**Don't compete with the stepper's `current`.** When a screen is
inside a step, the screen's own `step-header` should not introduce a
*second* magenta heading on the same vertical axis as the stepper's
current cell. Pick one focal heading per screen.

**Don't render auxiliary actions above the option list.** The auxiliary
panel always sits below, separated by a single muted rule. Above-the-list
placement competes with the cursor flow and forces the user to scan past
non-focusable rows before reaching the choices that matter.

**Don't add a component without adding tokens here first.** If a future
wizard step needs an atom that isn't in this list, update DESIGN.md
before writing the view. Drift starts the day a view defines its own
spacing.
