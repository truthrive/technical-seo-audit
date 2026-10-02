package standalone

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
