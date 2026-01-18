package api

import (
	"encoding/json"
	"time"
)

type CreateJobResponse struct {
	JobID string `json:"job_id"`
	State string `json:"state"`
}

type JobResponse struct {
	JobID          string          `json:"job_id"`
	State          string          `json:"state"`
	Payload        json.RawMessage `json:"payload"`
	MaxAttempts    int             `json:"max_attempts"`
	CurrentAttempt int             `json:"current_attempt"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	LastError      *string         `json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	CancelledAt    *time.Time      `json:"cancelled_at,omitempty"`
}

type ListJobsResponse struct {
	Jobs []JobResponse `json:"jobs"`
}
