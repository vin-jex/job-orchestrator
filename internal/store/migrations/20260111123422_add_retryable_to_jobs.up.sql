ALTER TABLE jobs
ADD COLUMN IF NOT EXISTS retryable BOOLEAN;

UPDATE jobs
SET
  retryable = FALSE
WHERE
  state IN ('COMPLETED', 'CANCELLED');

UPDATE jobs
SET
  retryable = TRUE
WHERE
  state = 'FAILED'
  AND current_attempt < max_attempts;

UPDATE jobs
SET
  retryable = FALSE
WHERE
  state = 'FAILED'
  AND current_attempt >= max_attempts;

ALTER TABLE jobs ADD CONSTRAINT retryable_only_on_failed CHECK (
  retryable IS NULL
  OR state = 'FAILED'
);