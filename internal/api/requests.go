package api

type CreateJobRequest struct {
	Type           string         `json:"type"`
	Payload        map[string]any `json:"payload"`
	MaxAttempts    int            `json:"max_attempts"`
	TimeoutSeconds int            `json:"timeout_seconds"`
}
