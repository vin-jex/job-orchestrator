package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vin-jex/job-orchestrator/internal/store"
)

// handleHealth godoc
// @Summary      Liveness probe
// @Description  Indicates whether the process is alive
// @Tags         ops
// @Produce      text/plain
// @Success      200 {string} string "ok"
// @Router       /healthz [get]
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleReady godoc
// @Summary      Readiness probe
// @Description  Indicates whether the service can accept traffic
// @Tags         ops
// @Produce      text/plain
// @Success      200 {string} string "ready"
// @Failure      503 {string} string "not ready"
// @Router       /readyz [get]
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := s.store.Ping(ctx); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

// handleMetrics godoc
// @Summary      Prometheus metrics
// @Description  Exposes service metrics in Prometheus format
// @Tags         ops
// @Produce      text/plain
// @Success      200 {string} string
// @Router       /metrics [get]
func (s *Server) handleMetrics() http.Handler {
	return promhttp.Handler()
}

// @Summary Create a new job
// @Description Create a job in the PENDING state
// @Tags Jobs
// @Accept json
// @Produce json
// @Param request body CreateJobRequest true "Job creation payload"
// @Success 201 {object} CreateJobResponse
// @Failure 400 {string} string
// @Failure 500 {string} string
// @Router /v1/jobs [post]
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

	LoggerFromContext(request.Context()).Info("job created", "job_id", jobID.String())
	response := CreateJobResponse{
		JobID: jobID.String(),
		State: "PENDING",
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(response)
}

// @Summary Cancel a job
// @Description Cancel a job in any non-terminal state. Cancellation is idempotent.
// @Tags Jobs
// @Param jobID path string true "Job ID"
// @Success 200
// @Failure 400 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /v1/jobs/{jobID}/cancel [post]
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
	LoggerFromContext(request.Context()).Info("job cancelled", "job_id", jobID.String())

	writer.WriteHeader(http.StatusOK)
}

// @Summary Get job details
// @Description Fetch the authoritative state and metadata of a job
// @Tags Jobs
// @Produce json
// @Param jobID path string true "Job ID"
// @Success 200 {object} JobResponse
// @Failure 400 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /v1/jobs/{jobID} [get]
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

// @Summary List jobs
// @Description List jobs with optional state filtering and limit
// @Tags Jobs
// @Produce json
// @Param state query string false "Filter by job state"
// @Param limit query int false "Maximum number of jobs (default 100)"
// @Success 200 {object} ListJobsResponse
// @Failure 500 {string} string
// @Router /v1/jobs [get]
func (s *Server) handleListJobs(
	writer http.ResponseWriter,
	request *http.Request,
) {
	query := request.URL.Query()

	var (
		state *string
		limit = 100
	)

	if rawState := query.Get("state"); rawState != "" {
		state = &rawState
	}

	if rawLimit := query.Get("limit"); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	jobs, err := s.store.ListJobs(request.Context(), state, limit)
	if err != nil {
		http.Error(writer, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	response := ListJobsResponse{
		Jobs: make([]JobResponse, 0, len(jobs)),
	}

	for _, job := range jobs {
		response.Jobs = append(response.Jobs, JobResponse{
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
		})
	}

	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

// @Summary Acquire job lease
// @Description Scheduler-only endpoint to atomically lease a PENDING job
// @Tags Internal-Scheduler
// @Accept json
// @Produce json
// @Param request body AcquireLeaseRequest true "Lease request"
// @Success 200 {object} AcquireLeaseResponse
// @Success 204 "No jobs available"
// @Failure 400 {string} string
// @Failure 500 {string} string
// @Router /internal/jobs/lease [post]
func (s *Server) handleAcquireLease(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var req AcquireLeaseRequest

	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		http.Error(writer, "invalid JSON body", http.StatusBadRequest)
		return
	}

	schedulerID, err := uuid.Parse(req.SchedulerID)
	if err != nil {
		http.Error(writer, "invalid scheduler_id", http.StatusBadRequest)
		return
	}

	if req.LeaseDurationSeconds <= 0 {
		http.Error(writer, "invalid lease duration", http.StatusBadRequest)
		return
	}

	jobID, expiresAt, err := s.store.AcquireJobLease(request.Context(), schedulerID, time.Duration(req.LeaseDurationSeconds)*time.Second)
	if err != nil {
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	response := AcquireLeaseResponse{
		JobID:          jobID.String(),
		LeaseExpiresAt: expiresAt,
	}

	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

// @Summary Recover expired leases
// @Description Trigger asynchronous recovery of expired job leases
// @Tags Internal-Scheduler
// @Success 202 "Recovery triggered"
// @Router /internal/jobs/recover [post]
func (s *Server) handleRecoverLeases(
	writer http.ResponseWriter,
	request *http.Request,
) {
	go func() {
		if _, err := s.store.RecoverExpiredLeases(
			context.Background(),
			time.Now(),
		); err != nil {
			s.logger.Error("lease recovery failed", "err", err)
		}
	}()

	writer.WriteHeader(http.StatusAccepted)
}

// @Summary Start job execution
// @Description Transition a job from SCHEDULED to RUNNING
// @Tags Internal-Worker
// @Produce json
// @Param jobID path string true "Job ID"
// @Success 200 {object} StartJobResponse
// @Failure 400 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /internal/jobs/{jobID}/start [post]
func (s *Server) handleStartJob(
	writer http.ResponseWriter,
	request *http.Request,
) {
	jobIDParam := request.PathValue("jobID")

	jobID, err := uuid.Parse(jobIDParam)
	if err != nil {
		http.Error(writer, "invalid job id", http.StatusBadRequest)
		return
	}

	err = s.store.MarkJobRunning(request.Context(), jobID)
	if err != nil {
		if err == store.ErrInvalidStateTransition {
			http.Error(writer, "job cannot be started", http.StatusConflict)
			return
		}

		http.Error(writer, "failed to start job", http.StatusInternalServerError)
		return
	}

	response := StartJobResponse{
		JobID: jobID.String(),
		State: store.JobRunning,
	}

	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

// @Summary Complete job
// @Description Mark a RUNNING job as COMPLETED
// @Tags Internal-Worker
// @Produce json
// @Param jobID path string true "Job ID"
// @Success 200 {object} CompleteJobResponse
// @Failure 400 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /internal/jobs/{jobID}/complete [post]
func (s *Server) handleCompleteJob(
	writer http.ResponseWriter,
	request *http.Request,
) {
	jobIDParam := request.PathValue("jobID")

	jobID, err := uuid.Parse(jobIDParam)
	if err != nil {
		http.Error(writer, "invalid job id", http.StatusBadRequest)
		return
	}

	err = s.store.CompleteJob(request.Context(), jobID)
	if err != nil {
		if err == store.ErrInvalidStateTransition {
			http.Error(writer, "job cannot be completed", http.StatusConflict)
			return
		}

		http.Error(writer, "failed to complete job", http.StatusInternalServerError)
		return
	}

	response := CompleteJobResponse{
		JobID: jobID.String(),
		State: store.JobCompleted,
	}

	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

// @Summary Fail job
// @Description Mark a RUNNING job as FAILED with retry semantics
// @Tags Internal-Worker
// @Accept json
// @Produce json
// @Param jobID path string true "Job ID"
// @Param request body FailJobRequest true "Failure details"
// @Success 200 {object} FailJobResponse
// @Failure 400 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /internal/jobs/{jobID}/fail [post]
func (s *Server) handleFailJob(
	writer http.ResponseWriter,
	request *http.Request,
) {
	jobIDParam := request.PathValue("jobID")

	jobID, err := uuid.Parse(jobIDParam)
	if err != nil {
		http.Error(writer, "invalid job id", http.StatusBadRequest)
		return
	}

	var failRequest FailJobRequest
	if err := json.NewDecoder(request.Body).Decode(&failRequest); err != nil {
		http.Error(writer, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if failRequest.Error == "" {
		http.Error(writer, "error message required", http.StatusBadRequest)
		return
	}

	err = s.store.FailJob(
		request.Context(),
		jobID,
		failRequest.Error,
		failRequest.Retryable,
	)
	if err != nil {
		if err == store.ErrInvalidStateTransition {
			http.Error(writer, "job cannot be failed", http.StatusConflict)
			return
		}

		http.Error(writer, "failed to mark job failed", http.StatusInternalServerError)
		return
	}

	response := FailJobResponse{
		JobID: jobID.String(),
		State: store.JobFailed,
	}

	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

// @Summary Worker heartbeat
// @Description Record worker liveness for lease safety
// @Tags Internal-Worker
// @Param workerID path string true "Worker ID"
// @Success 204
// @Failure 400 {string} string
// @Failure 500 {string} string
// @Router /internal/workers/{workerID}/heartbeat [post]
func (s *Server) handleWorkerHeartbeat(
	writer http.ResponseWriter,
	request *http.Request,
) {
	workerIDParam := request.PathValue("workerID")

	workerID, err := uuid.Parse(workerIDParam)
	if err != nil {
		http.Error(writer, "invalid worker id", http.StatusBadRequest)
		return
	}

	if err := s.store.HeartbeatWorker(request.Context(), workerID); err != nil {
		http.Error(writer, "failed to record heartbeat", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}
