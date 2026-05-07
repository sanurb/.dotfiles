package envelope

import (
	"io"
	"time"
)

// Stream wires the NDJSON stream for one long-running verb invocation.
// It owns the run_id, the wall-clock for event timestamps, and the
// per-step duration tracking; callers only describe phase boundaries
// (StepStarted / StepCompleted / StepFailed) and call Success or
// Failure to terminate.
//
// The clock is injectable so golden-transcript tests can pin
// timestamps without sleeping. Production callers pass time.Now.
type Stream struct {
	out     io.Writer
	command string
	runID   string
	logPath string
	now     func() time.Time

	// Per-step bookkeeping for duration_ms. Reset on each
	// StepStarted; consumed and zeroed on each StepCompleted or
	// StepFailed.
	stepName  string
	stepStart time.Time
}

// NewStream constructs a streamer. logPath is recorded once so the
// terminal envelope can carry it without callers tracking it
// separately. now defaults to time.Now when nil — tests pass a fixed
// clock for deterministic golden output.
func NewStream(out io.Writer, command, runID, logPath string, now func() time.Time) *Stream {
	if now == nil {
		now = time.Now
	}
	return &Stream{
		out:     out,
		command: command,
		runID:   runID,
		logPath: logPath,
		now:     now,
	}
}

// RunID returns the run identifier the streamer is tagging events
// with. Useful for callers that need to tag log lines or filenames
// with the same id the consumer sees on the wire.
func (s *Stream) RunID() string { return s.runID }

// LogPath returns the per-run log file path the terminal envelope
// will reference. Callers tee subprocess stderr to this path so the
// stream stays compact and the diagnostic detail lives in the file.
func (s *Stream) LogPath() string { return s.logPath }

// Start emits the opening event of the stream. Idempotent in the
// sense that a duplicate Start would simply emit a duplicate line —
// the protocol does not require uniqueness, only ordering.
func (s *Stream) Start() error {
	return EmitEvent(s.out, StreamEvent{
		Type:      EventStart,
		Timestamp: s.now(),
		Command:   s.command,
	})
}

// StepStarted records the wall-clock start of a phase and emits the
// corresponding step event. A subsequent StepCompleted / StepFailed
// computes duration_ms from the recorded start.
func (s *Stream) StepStarted(name string) error {
	s.stepName = name
	s.stepStart = s.now()
	return EmitEvent(s.out, StreamEvent{
		Type:      EventStep,
		Timestamp: s.stepStart,
		Name:      name,
		Status:    StepStarted,
	})
}

// StepCompleted emits a successful phase boundary with elapsed
// duration. Safe to call without a matching StepStarted; in that
// case duration_ms emits zero.
func (s *Stream) StepCompleted(name string) error {
	end := s.now()
	dur := s.elapsed(end)
	s.stepName, s.stepStart = "", time.Time{}
	return EmitEvent(s.out, StreamEvent{
		Type:       EventStep,
		Timestamp:  end,
		Name:       name,
		Status:     StepCompleted,
		DurationMs: dur,
	})
}

// StepFailed emits a failed phase boundary. The step event itself
// carries no error detail — the terminal error envelope is the prose
// surface, with code, retryable, user_action, and fix. Failure() is
// the natural follow-up.
func (s *Stream) StepFailed(name string) error {
	end := s.now()
	dur := s.elapsed(end)
	s.stepName, s.stepStart = "", time.Time{}
	return EmitEvent(s.out, StreamEvent{
		Type:       EventStep,
		Timestamp:  end,
		Name:       name,
		Status:     StepFailed,
		DurationMs: dur,
	})
}

// Success emits the terminal success envelope. After this, no more
// events should be written — the consumer's `jq -s 'last'` is
// already pointed at this line.
func (s *Stream) Success(result any, next []Action) error {
	return OKLong(s.out, s.command, s.runID, s.logPath, result, next)
}

// Failure emits the terminal error envelope. p's RunID and LogPath
// are populated from the stream's recorded values so callers don't
// need to thread them through every error path.
func (s *Stream) Failure(p *Problem) error {
	if p.RunID == "" {
		p.RunID = s.runID
	}
	if p.LogPath == "" {
		p.LogPath = s.logPath
	}
	return Fail(s.out, s.command, p)
}

func (s *Stream) elapsed(end time.Time) int64 {
	if s.stepStart.IsZero() {
		return 0
	}
	return end.Sub(s.stepStart).Milliseconds()
}
