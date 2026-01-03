package api

import (
	"encoding/json"
	"net/http"

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
