package api

import (
	"encoding/json"
	"net/http"

	"github.com/Vin-Jex/job-orchestrator/internal/store"
	"github.com/google/uuid"
)

func (s *Server) handleCreateJob(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var createRequest CreateJobRequest

	if err := json.NewDecoder(request.Body).Decode(&createRequest); err != nil {
		http.Error(writer, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if createRequest.MaxAttempts < 1 || createRequest.TimeoutSeconds <= 0 {
		http.Error(writer, "invalid job parameters", http.StatusBadRequest)
		return
	}

	jobID := uuid.New()

	payloadBytes, err := json.Marshal(createRequest.Payload)
	if err != nil {
		http.Error(writer, "invalid payload", http.StatusBadRequest)
		return
	}

	err = s.store.CreateJob(
		request.Context(),
		jobID,
		payloadBytes,
		createRequest.MaxAttempts,
		createRequest.TimeoutSeconds,
	)
	if err != nil {
		http.Error(writer, "failed to create job", http.StatusInternalServerError)
		return
	}

	response := CreateJobResponse{
		JobID: jobID.String(),
		State: "PENDING",
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(response)
}

func (s *Server) handleCancelJob(
	writer http.ResponseWriter,
	request *http.Request,
) {
	jobIDParam := request.PathValue("jobID")
	jobID, err := uuid.Parse(jobIDParam)

	if err != nil {
		http.Error(writer, "Invalid job id", http.StatusBadRequest)
		return
	}

	err = s.store.CancelJob(request.Context(), jobID)
	if err != nil {
		if err == store.ErrInvalidStateTransition {
			http.Error(writer, "Job cannot be cancelled", http.StatusConflict)
			return
		}

		http.Error(writer, "Failed to cancel job", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetJob(
	writer http.ResponseWriter,
	request *http.Request,
) {
	jobIDParam := request.PathValue("jobID")
	jobID, err := uuid.Parse(jobIDParam)
	if err != nil {
		http.Error(writer, "Invalid job id", http.StatusBadRequest)
		return
	}
	job, err := s.store.GetJobByID(request.Context(), jobID)
	if err != nil {
		http.Error(writer, "Failed to fetch job", http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(writer, "Job not found", http.StatusNotFound)
		return
	}

	response := JobResponse{
		JobID:          job.ID.String(),
		State:          job.State,
		Payload:        job.Payload,
		MaxAttempts:    job.MaxAttempts,
		CurrentAttempt: job.CurrentAttempt,
		TimeoutSeconds: job.TimeoutSeconds,
		LastError:      job.LastError,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		CancelledAt:    job.CancelledAt,
	}

	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}
