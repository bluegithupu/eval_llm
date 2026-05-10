# Cross-Area Behavioral Assertions for LLM Evaluation Backend

This document enumerates behavioral assertions that span multiple surfaces (API, DB, Redis, K8s Jobs, OpenCompass) for the LLM evaluation backend system.

---

## Flow 1: Complete Evaluation Flow

### VAL-CROSS-001: Full Evaluation Lifecycle End-to-End
When a user creates an evaluation via API, the system must successfully orchestrate the entire flow: API creates task → K8s Job spawns → OpenCompass executes → Results stored in DB → API returns paginated results. The evaluation status must transition through pending → running → completed, and all predictions must be retrievable with correct pagination.
**Tool:** curl (API) + kubectl (K8s) + psql (DB) + redis-cli (Redis)
**Evidence:**
- API: `curl -X POST /api/v1/tasks` returns 202 with task_id
- K8s: `kubectl get jobs -l task-id=<id>` shows Job created
- Redis: `redis-cli GET task:<id>:status` shows "running" during execution
- DB: `SELECT * FROM evaluations WHERE id='<id>'` shows completed status and predictions JSON
- API: `curl /api/v1/tasks/<id>/results?page=1&limit=20` returns correct paginated predictions

### VAL-CROSS-002: Task Creation Persists Across All Surfaces
When a task is created via API, the record must exist in PostgreSQL with status 'pending', a corresponding entry in Redis for status tracking, and no K8s Job created until explicitly started. All surfaces must have consistent task_id references.
**Tool:** curl + psql + redis-cli + kubectl
**Evidence:**
- API: `curl -X POST /api/v1/tasks -d '{"model_id":"test","datasets":["mmlu"]}'` returns task_id
- DB: `SELECT id, status, model_id FROM evaluations WHERE id='<task_id>'` shows pending status
- Redis: `redis-cli KEYS "*<task_id>*"` shows status key exists
- K8s: `kubectl get jobs -l task-id=<task_id>` shows no resources found

### VAL-CROSS-003: Evaluation Status Reflects Real Job State
When K8s Job transitions from Running to Completed, the API status endpoint must reflect the same state within polling intervals, and the DB updated_at timestamp must change accordingly. Status synchronization must be eventual but consistent.
**Tool:** kubectl + curl + psql
**Evidence:**
- K8s: `kubectl get job <job-name> -o jsonpath='{.status.conditions[0].type}'` shows "Complete"
- API: `curl /api/v1/tasks/<id>/status` shows "completed"
- DB: `SELECT status, updated_at FROM evaluations WHERE id='<id>'` shows completed and recent timestamp

### VAL-CROSS-004: Results Persist Correctly After Job Completion
When OpenCompass writes evaluation results to JSON files, the K8s Job must copy them to persistent storage, the DB must have predictions inserted with correct evaluation_id foreign key, and the API must return paginated results matching the stored data exactly.
**Tool:** curl + psql + kubectl logs
**Evidence:**
- K8s: `kubectl logs job/<job-name>` shows "Results written to /results/eval_<id>.json"
- DB: `SELECT COUNT(*) FROM predictions WHERE evaluation_id='<id>'` matches expected count
- API: `curl /api/v1/tasks/<id>/results` returns predictions with correct evaluation_id

---

## Flow 2: Task Cancellation

### VAL-CROSS-005: Cancellation Terminates Job and Updates Status
When a user cancels a running evaluation via API DELETE, the K8s Job must be terminated (deleted), the DB status must update to 'cancelled', Redis cache must be cleaned up, and any partial results must be preserved in the database.
**Tool:** curl + kubectl + psql + redis-cli
**Evidence:**
- API: `curl -X DELETE /api/v1/tasks/<id>` returns 200 with cancellation confirmation
- K8s: `kubectl get job <job-name>` shows "Error from server (NotFound)" or Job status shows terminated
- DB: `SELECT status FROM evaluations WHERE id='<id>'` shows "cancelled"
- Redis: `redis-cli GET task:<id>:status` returns nil or shows "cancelled"
- DB: `SELECT COUNT(*) FROM predictions WHERE evaluation_id='<id>'` shows partial results if any were written

### VAL-CROSS-006: Cancellation Mid-Execution Preserves Partial Results
When cancellation occurs during OpenCompass execution (e.g., after 50% of dataset), the system must preserve any completed predictions in the DB before termination, mark the evaluation as 'cancelled', and ensure K8s Job cleanup completes gracefully.
**Tool:** curl + kubectl + psql
**Evidence:**
- API: `curl -X DELETE /api/v1/tasks/<running_id>` returns 200
- K8s: `kubectl logs job/<job-name> --previous` shows graceful shutdown initiated
- DB: `SELECT status, result_count FROM evaluations WHERE id='<id>'` shows "cancelled" with partial count > 0

### VAL-CROSS-007: Orphaned Job Detection and Cleanup
When a K8s Job exists but the corresponding DB record shows 'cancelled' or is missing, the system must detect the orphaned Job and clean it up, with the cleanup action logged and DB remaining consistent.
**Tool:** kubectl + psql + application logs
**Evidence:**
- K8s: `kubectl get jobs` lists job with label task-id=<orphan_id>
- DB: `SELECT * FROM evaluations WHERE id='<orphan_id>'` shows status='cancelled' or no rows
- Logs: Cleanup job logs show "Orphaned job <job-name> terminated for task <id>"

---

## Flow 3: Error Propagation

### VAL-CROSS-008: OpenCompass Failure Propagates to API and DB
When OpenCompass execution fails (non-zero exit code, exception, or timeout), the K8s Job must be marked as failed, the DB must record the error message and stack trace, the API status must return 'failed' with error details, and Redis status must reflect failure.
**Tool:** kubectl + psql + curl + redis-cli
**Evidence:**
- K8s: `kubectl get job <job-name> -o jsonpath='{.status.conditions[?(@.type=="Failed")].message}'` shows error
- DB: `SELECT status, error_message, error_details FROM evaluations WHERE id='<id>'` shows "failed" with error info
- API: `curl /api/v1/tasks/<id>/status` returns {"status":"failed","error":"..."}
- Redis: `redis-cli GET task:<id>:status` shows "failed"

### VAL-CROSS-009: K8s Job Creation Failure Returns Meaningful Error
When K8s Job creation fails (e.g., insufficient resources, quota exceeded, invalid config), the API must return 503 with descriptive error, the DB must show 'failed' status with creation_error, and no orphaned resources must remain.
**Tool:** curl + kubectl + psql
**Evidence:**
- API: `curl -X POST /api/v1/tasks -d '{"model_id":"large-model"}'` returns 503 with error message
- K8s: `kubectl get jobs -l task-id=<id>` shows no resources
- DB: `SELECT status, error_message FROM evaluations WHERE id='<id>'` shows "failed" with "Job creation failed: insufficient resources"

### VAL-CROSS-010: Database Connection Failure Degrades Gracefully
When PostgreSQL becomes unavailable during evaluation, the API must return 503, running K8s Jobs must continue (results buffered locally), and upon DB recovery, buffered results must be persisted with correct timestamps.
**Tool:** curl + psql + kubectl logs
**Evidence:**
- API: `curl /api/v1/tasks` returns 503 with "Database unavailable"
- K8s: `kubectl logs job/<job-name>` shows "Results buffered locally: /tmp/results_<id>.json"
- DB: After recovery, `SELECT * FROM evaluations WHERE id='<id>'` shows completed with results

### VAL-CROSS-011: Redis Failure Falls Back to DB Polling
When Redis is unavailable, the system must fall back to querying K8s Job status directly and update the DB, with API status endpoint still functioning (potentially slower), and recovery must sync Redis state from DB.
**Tool:** curl + redis-cli + kubectl + psql
**Evidence:**
- Redis: `redis-cli PING` shows "Could not connect to Redis"
- API: `curl /api/v1/tasks/<id>/status` still returns correct status (slower response)
- K8s: `kubectl get job <job-name> -o jsonpath='{.status}'` shows actual status
- After Redis recovery: `redis-cli GET task:<id>:status` matches DB status

---

## Flow 4: Status Synchronization

### VAL-CROSS-012: Redis Status Mirrors K8s Job Status
When K8s Job status changes (Pending → Running → Succeeded/Failed), Redis status key must be updated within the sync interval, and API polling must return consistent status. Stale reads must not exceed sync_interval tolerance.
**Tool:** kubectl + redis-cli + curl
**Evidence:**
- K8s: `kubectl get job <job-name> -o jsonpath='{.status.active}'` shows 1 (Running)
- Redis: `redis-cli GET task:<id>:status` shows "running"
- API: `curl /api/v1/tasks/<id>/status` returns {"status":"running","progress":45}

### VAL-CROSS-013: DB updated_at Changes on Any Status Update
When any status transition occurs (pending→running→completed/failed/cancelled), the DB updated_at timestamp must change, and the timestamp must be within 1 second of the actual transition time recorded in K8s events.
**Tool:** kubectl + psql
**Evidence:**
- K8s: `kubectl describe job <job-name>` shows event timestamps
- DB: `SELECT status, updated_at FROM evaluations WHERE id='<id>'` shows updated_at matching K8s event time within 1 second

### VAL-CROSS-014: Concurrent Status Queries Return Consistent Results
When multiple API clients poll the same task status simultaneously (10+ concurrent requests), all responses must return the same status value, and no stale or inconsistent states must be observed across any surface.
**Tool:** curl (parallel) + psql
**Evidence:**
- Run: `for i in {1..10}; do curl -s /api/v1/tasks/<id>/status & done; wait`
- Result: All 10 responses show identical status JSON
- DB: `SELECT status FROM evaluations WHERE id='<id>'` matches all API responses

---

## Flow 5: Model Key Flow

### VAL-CROSS-015: Model API Key Securely Mounted to Job
When an evaluation requires a model API key (e.g., OpenAI, Anthropic), the secret must be created in K8s, mounted as environment variable to the Job pod, OpenCompass must receive it without logging, and the key must not appear in API responses or DB records.
**Tool:** kubectl + curl + psql + kubectl logs
**Evidence:**
- K8s: `kubectl get secret model-key-<id> -o jsonpath='{.data.api_key}'` shows base64 encoded key
- K8s: `kubectl describe job <job-name>` shows env from secret
- K8s: `kubectl logs job/<job-name>` does NOT contain the API key anywhere
- API: `curl /api/v1/tasks/<id>` does NOT return model_api_key field
- DB: `SELECT * FROM evaluations WHERE id='<id>'` does NOT contain API key in any column

### VAL-CROSS-016: Secret Rotation Does Not Break Running Evaluations
When a model API key is rotated (secret updated in K8s), running Jobs must continue with the original key (pod environment is immutable), new Jobs must receive the new key, and failed Jobs must retry with current key on restart.
**Tool:** kubectl + curl
**Evidence:**
- K8s: Update secret: `kubectl patch secret model-key-<id> ...`
- K8s: `kubectl logs job/<running-job>` shows evaluation continues successfully
- API: Create new task with same model, `curl /api/v1/tasks/<new-id>/status` shows success
- K8s: `kubectl exec job/<new-job> -- env | grep API_KEY` shows new key value

---

## Flow 6: Result Persistence

### VAL-CROSS-017: Large Result Set Paginated Correctly
When an evaluation produces 10,000+ predictions, the API must paginate results correctly (page=1 limit=100 returns first 100, page=2 returns next 100), the DB query must use OFFSET/LIMIT efficiently, and total count must be accurate.
**Tool:** curl + psql
**Evidence:**
- API: `curl /api/v1/tasks/<id>/results?page=1&limit=100` returns 100 items, page=2 returns next 100
- DB: `EXPLAIN ANALYZE SELECT * FROM predictions WHERE evaluation_id='<id>' LIMIT 100 OFFSET 100` shows efficient query plan
- API: `curl /api/v1/tasks/<id>/results` returns {"total": 10500, "pages": 105}

### VAL-CROSS-018: Result Integrity After Completion
When results are stored, the predictions count in DB must match OpenCompass output JSON count exactly, the score/aggregation in DB must match the computed scores from predictions, and API response must match DB data byte-for-byte.
**Tool:** kubectl exec + psql + curl + jq
**Evidence:**
- K8s: `kubectl exec job/<job-name> -- cat /results/eval_<id>.json | jq '.predictions | length'` shows 5000
- DB: `SELECT COUNT(*) FROM predictions WHERE evaluation_id='<id>'` shows 5000
- API: `curl /api/v1/tasks/<id>/results | jq 'length'` shows 5000
- DB: `SELECT accuracy_score FROM evaluations WHERE id='<id>'` matches computed score from predictions

### VAL-CROSS-019: Concurrent Evaluations Isolated Correctly
When 5+ evaluations run simultaneously (different models, same or different datasets), each evaluation must have isolated K8s Job, isolated DB records, isolated result storage, and no cross-contamination of predictions or status.
**Tool:** curl + kubectl + psql
**Evidence:**
- API: Create 5 tasks: `for i in {1..5}; do curl -X POST /api/v1/tasks -d "{\"model_id\":\"model_$i\"}" & done`
- K8s: `kubectl get jobs -l eval-id` shows 5 distinct jobs
- DB: `SELECT id, model_id FROM evaluations WHERE id IN (...)` shows 5 distinct rows
- DB: `SELECT evaluation_id, COUNT(*) FROM predictions GROUP BY evaluation_id` shows no overlap

### VAL-CROSS-020: Resource Cleanup After Completion
After evaluation completion (success or failure), K8s Jobs older than retention period must be cleaned up, K8s Pods must be removed (or retained per policy), Redis keys must be expired per TTL, and DB records must remain for historical queries.
**Tool:** kubectl + redis-cli + psql
**Evidence:**
- K8s: `kubectl get jobs -l created-before=<retention-cutoff>` shows no old jobs
- K8s: `kubectl get pods -l job-name=<old-job>` shows no pods
- Redis: `redis-cli TTL task:<old-id>:status` shows remaining TTL or -2 (expired)
- DB: `SELECT * FROM evaluations WHERE id='<old-id>'` still returns the record

---

## Flow 7: Concurrency and Timing

### VAL-CROSS-021: Race Condition on Duplicate Task Creation
When two API requests create a task with identical model_id and datasets simultaneously, the system must either reject one as duplicate (409 Conflict) or create two separate tasks with unique IDs, never corrupting shared state.
**Tool:** curl (parallel) + psql
**Evidence:**
- Run: `curl -X POST /api/v1/tasks -d '{"model_id":"gpt-4","datasets":["mmlu"]}' & curl -X POST /api/v1/tasks -d '{"model_id":"gpt-4","datasets":["mmlu"]}' & wait`
- Result: Either one returns 409 with error, or both return 202 with distinct task_ids
- DB: `SELECT COUNT(*) FROM evaluations WHERE model_id='gpt-4' AND datasets @> '["mmlu"]'` shows 1 or 2 rows (not corrupted)

### VAL-CROSS-022: Status Update Race Condition Prevention
When Job completion and user cancellation occur nearly simultaneously, the final status must be deterministic (either 'completed' or 'cancelled', never both), and DB must reflect exactly one terminal state.
**Tool:** curl + kubectl + psql
**Evidence:**
- Scenario: Job 99% complete, user sends DELETE at same time
- Result: DB shows exactly one terminal status (not 'completed_cancelled' or ambiguous)
- K8s: Job shows either Completed or was deleted
- DB: `SELECT status FROM evaluations WHERE id='<id>'` shows deterministic final state

### VAL-CROSS-023: High Concurrency Load Handling
When 20 concurrent evaluations are created within 1 second, all 20 must be accepted by API (or queued if rate-limited), 20 distinct K8s Jobs must be created (or queued), and all 20 must complete successfully without resource starvation.
**Tool:** curl (parallel) + kubectl + psql
**Evidence:**
- Run: `for i in {1..20}; do curl -X POST /api/v1/tasks -d "{\"model_id\":\"model_$i\"}" & done; wait`
- API: All 20 requests return 202 (or 429 if rate-limited with Retry-After)
- K8s: `kubectl get jobs | wc -l` shows 20 new jobs
- DB: `SELECT COUNT(*) FROM evaluations WHERE created_at > NOW() - INTERVAL '1 minute'` shows 20
- Final: All 20 show completed status after sufficient wait

---

## Flow 8: Timing and Ordering

### VAL-CROSS-024: Status Transition Order Enforced
Evaluation status must only transition in valid order: pending → running → (completed | failed | cancelled). Direct transitions like pending → completed or running → pending must be rejected.
**Tool:** psql + curl (with invalid state manipulation attempt)
**Evidence:**
- Attempt: Direct DB update to skip running: `UPDATE evaluations SET status='completed' WHERE status='pending' AND id='<id>'`
- Result: Constraint violation OR trigger prevents invalid transition
- API: Status endpoint shows valid state (running if Job started, not completed)

### VAL-CROSS-025: Dependent Operations Execute in Correct Order
When creating an evaluation that depends on a model key secret, the secret must be created before K8s Job, K8s Job must not start until secret exists, and API must return error if secret creation fails (not start Job anyway).
**Tool:** curl + kubectl + psql
**Evidence:**
- API: Create task requiring model key: `curl -X POST /api/v1/tasks -d '{"model_id":"gpt-4"}'`
- K8s: `kubectl get secret model-key-<id>` exists BEFORE job starts
- K8s: `kubectl describe job <job-name>` shows secret mounted
- If secret fails: API returns 500/503, DB shows failed with "secret creation error"

---

## Summary Table

| ID | Flow Area | Assertion |
|----|-----------|-----------|
| VAL-CROSS-001 | Complete Evaluation | Full lifecycle end-to-end |
| VAL-CROSS-002 | Complete Evaluation | Task creation persists across all surfaces |
| VAL-CROSS-003 | Status Sync | Status reflects real Job state |
| VAL-CROSS-004 | Result Persistence | Results persist correctly after completion |
| VAL-CROSS-005 | Cancellation | Cancellation terminates Job and updates status |
| VAL-CROSS-006 | Cancellation | Partial results preserved on mid-execution cancel |
| VAL-CROSS-007 | Cancellation | Orphaned Job detection and cleanup |
| VAL-CROSS-008 | Error Propagation | OpenCompass failure propagates to API/DB |
| VAL-CROSS-009 | Error Propagation | K8s Job creation failure returns meaningful error |
| VAL-CROSS-010 | Error Propagation | DB connection failure degrades gracefully |
| VAL-CROSS-011 | Error Propagation | Redis failure falls back to DB polling |
| VAL-CROSS-012 | Status Sync | Redis status mirrors K8s Job status |
| VAL-CROSS-013 | Status Sync | DB updated_at changes on status update |
| VAL-CROSS-014 | Status Sync | Concurrent status queries return consistent results |
| VAL-CROSS-015 | Model Key | Model API key securely mounted to Job |
| VAL-CROSS-016 | Model Key | Secret rotation does not break running evaluations |
| VAL-CROSS-017 | Result Persistence | Large result set paginated correctly |
| VAL-CROSS-018 | Result Persistence | Result integrity after completion |
| VAL-CROSS-019 | Concurrency | Concurrent evaluations isolated correctly |
| VAL-CROSS-020 | Result Persistence | Resource cleanup after completion |
| VAL-CROSS-021 | Concurrency | Race condition on duplicate task creation |
| VAL-CROSS-022 | Concurrency | Status update race condition prevention |
| VAL-CROSS-023 | Concurrency | High concurrency load handling |
| VAL-CROSS-024 | Timing/Ordering | Status transition order enforced |
| VAL-CROSS-025 | Timing/Ordering | Dependent operations execute in correct order |

---

*Document generated for LLM Evaluation Backend contract testing.*
