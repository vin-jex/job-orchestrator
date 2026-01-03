DROP INDEX IF EXISTS idx_workers_heartbeat;
DROP INDEX IF EXISTS idx_job_leases_expires_at;
DROP INDEX IF EXISTS idx_jobs_state_created_at;

DROP TABLE IF EXISTS workers;
DROP TABLE IF EXISTS job_leases;
DROP TABLE IF EXISTS jobs;

DROP TYPE IF EXISTS job_state;