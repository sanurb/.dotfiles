# The TUI exits before realization; `main.go` re-execs `dots apply`

The wizard runs inside bubbletea's alt-screen. On consent to realize,
the wizard exits with `Result.RealizeRequested = true` and `main.go`
re-execs the dots binary as `dots apply --yes` so the realization
driver (`nh home switch`) renders against the real terminal. Embedding
`nh` inside the alt-screen would either swallow its progress UI or
require maintaining a second renderer for it.

## Consequences

"Stream live realization output inside the wizard" is the obvious
refactor and the wrong one — it pulls the alt-screen renderer back
into the activation path. The wizard signals consent; it does not
drive `nh`.
