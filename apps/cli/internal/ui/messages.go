package ui

import "time"

// Internal message types. The UI emits tea.Cmd functions that perform
// blocking work via the injected ports, then return one of these
// messages back to Update. View logic never blocks; execution logic
// never touches View. This is the contract.

type scanCompleteMsg struct {
	collisions []Collision
}

type scanFailedMsg struct{ err error }

type snapshotCompleteMsg SnapshotResult

type snapshotFailedMsg struct{ err error }

type realizeCompleteMsg RealizationResult

type realizeFailedMsg struct{ err error }

// progressTickMsg drives the synthetic progress bar during Realize.
// moon/nix don't emit machine-readable progress events, so we ramp
// to 95% over an estimated duration and snap to 100% on completion.
type progressTickMsg time.Time
