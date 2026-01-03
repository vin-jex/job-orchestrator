CREATE TYPE job_state AS ENUM (
  'PENDING',
  'SCHEDULED',
  'RUNNING',
  'COMPLETED',
  'FAILED',
  'CANCELLED'
);

CREATE TABLE
  jobs (
    id UUID PRIMARY KEY,
    state job_state NOT NULL,
    payload JSONB NOT NULL,
    max_attempts INTEGER NOT NULL CHECK (max_attempts >= 1),
    current_attempt INTEGER NOT NULL CHECK (current_attempt >= 0),
    timeout_seconds INTEGER NOT NULL CHECK (timeout_seconds > 0),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now (),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now (),
    cancelled_at TIMESTAMPTZ,
    CONSTRAINT terminal_state_no_retry CHECK (
      NOT (
        state IN ('COMPLETED', 'CANCELLED')
        AND current_attempt > 0
      )
    )
  );

CREATE TABLE
  job_leases (
    job_id UUID PRIMARY KEY REFERENCES jobs (id) ON DELETE CASCADE,
    scheduler_id UUID NOT NULL,
    lease_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now ()
  );

CREATE TABLE
  workers (
    id UUID PRIMARY KEY,
    last_heartbeat TIMESTAMPTZ NOT NULL,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now ()
  );

CREATE INDEX idx_jobs_state_created_at ON jobs (state, created_at);

CREATE INDEX idx_job_leases_expires_at ON job_leases (lease_expires_at);

CREATE INDEX idx_workers_heartbeat ON workers (last_heartbeat);