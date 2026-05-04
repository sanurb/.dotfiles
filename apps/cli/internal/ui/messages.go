package ui

// Internal message types. The UI emits tea.Cmd functions that perform
// blocking work via the injected ports, then return one of these
// messages back to Update. View logic never blocks; execution logic
// never touches View. This is the contract.
//
// Realize-completion messages are intentionally absent — the wizard no
// longer drives `nh home switch` from inside the bubbletea program.
// On user consent, the wizard returns a Result with RealizeRequested
// set; main.go then invokes `dots deploy` as a subprocess so nh's
// progress renders against the real terminal. See ADR-0009.

type scanCompleteMsg struct {
	collisions []Collision
}

type scanFailedMsg struct{ err error }

type snapshotCompleteMsg SnapshotResult

type snapshotFailedMsg struct{ err error }
