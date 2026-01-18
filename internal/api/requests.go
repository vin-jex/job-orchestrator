package api

type CreateJobRequest struct {
	Type           string         `json:"type"`
	Payload        map[string]any `json:"payload"`
	MaxAttempts    int            `json:"max_attempts"`
	TimeoutSeconds int            `json:"timeout_seconds"`
}

type FailJobRequest struct {
	Error     string `json:"error"`
	Retryable bool   `json:"retryable"`
}

type AcquireLeaseRequest struct {
	SchedulerID          string `json:"scheduler_id"`
	LeaseDurationSeconds int    `json:"lease_duration_seconds"`
}
