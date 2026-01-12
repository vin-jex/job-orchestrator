package store

import "fmt"

const (
	JobPending   = "PENDING"
	JobScheduled = "SCHEDULED"
	JobRunning   = "RUNNING"
	JobCompleted = "COMPLETED"
	JobFailed    = "FAILED"
	JobCancelled = "CANCELLED"
)

var terminalStates = map[string]bool{
	JobCompleted: true,
	JobFailed:    true,
	JobCancelled: true,
}

var allowedTransitions = map[string]map[string]bool{
	JobPending: {
		JobScheduled: true,
		JobCancelled: true,
	},
	JobScheduled: {
		JobRunning:   true,
		JobPending:   true,
		JobCancelled: true,
	},
	JobRunning: {
		JobCompleted: true,
		JobFailed:    true,
		JobCancelled: true,
	},
}

func ValidateJobTransition(from, to string) error {
	if terminalStates[from] {
		return fmt.Errorf("cannot transition from terminal state %s", from)
	}

	if allowed, ok := allowedTransitions[from][to]; !ok || !allowed {
		return fmt.Errorf("invalid job state transition from %s to %s", from, to)
	}

	return nil
}
