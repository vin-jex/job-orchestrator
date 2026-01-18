# Distributed Job Orchestrator (Go)

A correctness-first distributed job orchestration control plane implemented in Go.

This project focuses on **coordination, failure recovery, and strict state-machine enforcement** rather than application-level features or frameworks.
---

## What This System Guarantees

At all times, the system enforces the following invariants:

- A job is executed by **at most one worker**
- No job runs without an active lease
- Terminal job states are **immutable**
- All job state transitions are **validated and atomic**
- Scheduler and worker crashes **do not corrupt state**
- Multiple schedulers may run concurrently without coordination

If any of these guarantees are violated, it is a system bug.
---

## Architecture Overview

The system consists of four independent processes:

- **Control Plane**  
  Accepts jobs and enforces all state transitions via a strict state machine

- **Scheduler**  
  Acquires time-bound job leases and performs deterministic recovery

- **Workers**  
  Execute leased jobs concurrently while respecting cancellation and timeouts

- **PostgreSQL**  
  The single source of truth for coordination and recovery

All processes are **stateless and disposable**.  
Correctness is enforced entirely through transactional database logic.
---

## Job State Machine

Jobs move through a strictly enforced lifecycle:

```
PENDING → SCHEDULED → RUNNING → { COMPLETED | FAILED | CANCELLED }
FAILED → PENDING (retry, if attempts remain)
````

Invalid transitions are rejected.  
Terminal states (`COMPLETED`, `FAILED`, `CANCELLED`) never transition again.

All transitions are validated and tested.
---

## Lease-Based Coordination

- Schedulers acquire leases using transactional row locking
- Leases have explicit expiration timestamps
- Workers only execute jobs that are actively leased
- Expired leases are recovered deterministically
- No in-memory coordination is required

Schedulers safely compete using `FOR UPDATE SKIP LOCKED`.
---

## Failure Recovery

The system tolerates:

- Scheduler crashes
- Worker crashes
- Mid-execution termination
- Duplicate schedulers
- Network partitions between components

Recovery is driven entirely by persisted state, not process memory.
---

## Testing Philosophy

Tests validate **system invariants**, not timing.

Examples:
- Cancelling a job twice does not corrupt state
- Terminal jobs cannot re-enter execution
- Invalid transitions are rejected
- Retry limits are enforced deterministically

If a test passes due to timing luck, it is considered invalid.
---

## Explicit Non-Goals

This project intentionally does **not** include:

- UI dashboards
- Workflow DSLs
- Cron semantics
- Priority queues
- Swagger / OpenAPI
- HTTP middleware abstractions
- CORS handling

These features add surface area without improving correctness guarantees.
---

## Local Development

Requirements:
- Go 1.25.5+
- PostgreSQL

Start PostgreSQL:

```bash
docker run --name job-orchestrator-db \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=job_orchestrator \
  -p 5432:5432 \
  -d postgres:16
````

Run migrations:

```bash
migrate -path internal/store/migrations \
  -database "postgres://postgres:postgres@localhost:5432/job_orchestrator?sslmode=disable" up
```