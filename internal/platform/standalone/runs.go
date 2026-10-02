package standalone

import "time"

// Run states. Preserves the serialized run-state values established by SiteCrawl.
const (
	StateRunning     = "running"
	StatePaused      = "paused"
	StateCompleted   = "completed"
	StateCancelled   = "cancelled"
	StateStopped     = "stopped"
	StateFailed      = "failed"
	StateInterrupted = "interrupted"
	StateDeleting    = "deleting"
)

// IsTerminal reports whether the given run state represents a terminal (non-running, non-resumable) state.
func IsTerminal(state string) bool {
	switch state {
	case StateCompleted, StateCancelled, StateStopped, StateFailed, StateInterrupted:
		return true
	default:
		return false
	}
}

// Now returns the current UTC timestamp formatted as RFC3339.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
