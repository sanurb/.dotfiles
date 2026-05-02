# stepview — visual + interaction contract

The four legibility cues each get one and only one treatment. Future
step implementations consume this package and MUST NOT redefine them.

## Four state cues

- **Flow position.** Breadcrumb glyphs: `●` completed (success/green),
  `◉` current (accent/blue, bold), `○` upcoming (muted/gray). Paired
  with a left-aligned `Step N of M` counter on the same line. The
  breadcrumb also serves as the goal-gradient progress affordance.
- **Detected default.** Trailing `★` on the default row in warn/yellow.
  Shown exactly once per step — never echoed in the header.
- **Cursor.** Left-gutter `▸` arrow in accent/blue. Hovers freely; does
  not commit. Non-cursor rows hold the column with a space so rows
  stay aligned (Law of Common Region).
- **Selection.** Committed row carries a `[x]` checkbox prefix in
  cyan/bold and the row label is also cyan/bold. Uncommitted rows show
  `[ ]` in muted/gray. Cursor and selection are independent: any
  combination on the same or different rows is rendered without
  ambiguity (e.g. `▸ [x] macOS ★` stacks all three).

## Keyboard contract (Jakob's Law + Postel's Law)

`Keymap` is a typed struct rendered in a fixed slot order — Move,
Select, Back, Help, Quit — so the footer never reorders between steps.
`DefaultKeymap(includeBack bool)` is the canonical constructor:

- Step 1 calls `DefaultKeymap(false)`; Back is empty so the footer
  honestly omits it (no false affordance).
- Steps 2–6 call `DefaultKeymap(true)`.
- Step 6 sets `Keymap.SelectVerb = "confirm"` to relabel "select" →
  "confirm" without altering key positions.

The wizard host is expected to accept arrow keys *and* `k`/`j` for Move
(Postel's Law); the help overlay advertises both. Quit is required on
every step.

## Help overlay

`Model.Help = true` replaces the content slot with a key list derived
from the same `Keymap` that builds the footer. There is no second
source of truth for keys, so the overlay can never disagree with the
footer.

## Long lists (truncation/scroll cues)

Use `Viewport{Rows, Cursor, Height}` for any list that may exceed
visible space. It returns `(body, pos, total)`:

- A scrolled-from-top window prefixes `▲ N more above` (italic muted).
- A scrolled-from-bottom window suffixes `▼ N more below`.
- The caller passes `pos`/`total` into `Model.ListPos`/`ListTotal`,
  rendering `Title  (P/T)` next to the title so the user always knows
  how deep the list is and where the cursor sits within it.

Setting `Height = 0` disables truncation (short lists render in full).

## Empty / Loading / Error states

`Model.State` selects the content-slot rendering. Blank screens are
forbidden — every state has a glyph + readable message:

- `StateLoading` → `◐ <StatusMsg>` (accent/blue, bold). Default copy
  `"Loading…"`.
- `StateEmpty`   → muted italic `<StatusMsg>`. Default copy
  `"No items to show."`.
- `StateError`   → `✗ <StatusMsg>` (error/red, bold). Default copy
  `"Something went wrong."`.

Footer keys remain live in every state so the user can always Back or
Quit. `StateError` does not hijack input.

## Back navigation preserves context

`stepview` is purely declarative: every render is a pure function of
`Model`. The wizard host is contractually responsible for replaying
the same `Cursor`, `Selected`, and any list-scroll position when the
user navigates Back into a previously-visited step. The help overlay
states this explicitly to set expectation: *"Return to previous step
(selections preserved)"*.

## Reflow

Header uses `joinSpread`: counter on the left, breadcrumb on the right
of the row, padded to `Width`. If the terminal is too narrow to fit
both, the breadcrumb wraps to its own line — neither piece clips. The
80×24 design target sits comfortably within budget (counter ~11 cells,
breadcrumb ~11 cells).
