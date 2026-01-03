package api

type CreateJobResponse struct {
	JobID string `json:"job_id"`
	State string `json:"state"`
}
