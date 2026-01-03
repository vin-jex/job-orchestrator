# Job Orchestrator

A production-grade distributed job orchestrator platform written in Go.

This project focuses on correctness, coordination, and failure recovery rather than application-level features.

## Scope

- Distributed job scheduling using lease-based coordination
- Multiple schedulers and workers
- Crash-safe execution
- Explicit job state machine

## Non-goals

- Business-specific job semantics
- Workflow DSLs
- UI dashboard
- High-level framework

## Architecture Overview

Components: 

- Control Panel: API + state transition
- Scheduler: lease acquisition and recovery
- Workers: concurrent job execution
- PostgreSQL: source of truth

The database enforces correctness. Processes are disposable

## Job Lifecycle

Jobs move through a strict state machine:

PENDING => SCHEDULED => RUNNING { COMPLETED | FAILED | CANCELED }

Retries occur via FAILED => PENDING

Terminal states are immutable

## Local Development

Requirements: 
- Go 1.22+
- Docker
- PostgreSQL

Start PostgreSQL: 

```bash
docker run --name job-orchestrator-db \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=job_orchestrator \
  -p 5432:5432 \
  -d postgres:16
```

Run Migration: 
```bash
migrate -path internal/store/migrations \
  -database "postgres://postgres:postgres@localhost:5432/job_orchestrator?sslmode=disable" up
```


## Design Invariants

- Database is the source of truth
- No job runs without a lease
- A job runs at most one worker
- All state transitions are atomic
- Processes are stateless and disposable

