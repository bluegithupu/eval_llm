# Validation Contract: LLM Evaluation Backend

This document defines the behavioral assertions that constitute "done" for the LLM evaluation backend mission. Each assertion specifies a testable behavior with clear pass/fail criteria.

---

## Area: API Endpoints

### VAL-API-001: Create Evaluation Task Successfully
When a valid POST request is made to /api/v1/evaluations with required fields (model, dataset), the system creates a new evaluation task and returns HTTP 202 Accepted with a task ID in the response body.
Tool: curl
Evidence: Response status code 202, JSON body containing `task_id` field, `Location` header with task URL

### VAL-API-002: Create Task Returns Async Accepted Status
The endpoint returns HTTP 202 Accepted (not 201 Created) because evaluation tasks are processed asynchronously.
Tool: curl
Evidence: Response status code 202, response body includes `status: "pending"` or `status: "queued"`

### VAL-API-003: Missing Model Field Validation
When POST request is made without the `model` field, the system returns HTTP 400 Bad Request with an error message indicating the missing required field.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field containing "model" as required field

### VAL-API-004: Missing Dataset Field Validation
When POST request is made without the `dataset` field, the system returns HTTP 400 Bad Request with an error message indicating the missing required field.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field containing "dataset" as required field

### VAL-API-005: Invalid Model Value Validation
When POST request includes an unsupported `model` value (not in supported models list), the system returns HTTP 400 Bad Request with an error listing valid models.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field and list of valid model names (gpt-4, claude, qwen)

### VAL-API-006: Invalid Dataset Value Validation
When POST request includes an unsupported `dataset` value (not in supported datasets list), the system returns HTTP 400 Bad Request with an error listing valid datasets.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field and list of valid dataset names (MMLU, HellaSwag, etc.)

### VAL-API-007: Task ID Format Validation
The returned task ID must be a valid UUID v4 format string.
Tool: curl
Evidence: Response body contains `task_id` matching UUID v4 regex pattern `[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`

### VAL-API-008: List Tasks Default Pagination
When GET request is made without pagination parameters, the system returns default page (page=1) with default limit (e.g., 10 items).
Tool: curl
Evidence: Response status code 200, JSON body with `tasks` array, `page: 1`, `limit: 10`

### VAL-API-009: List Tasks Custom Pagination
When GET request includes `?page=2&limit=5`, the system returns the second page with 5 items per page.
Tool: curl
Evidence: Response status code 200, JSON body with `page: 2`, `limit: 5`, `tasks` array of max 5 items

### VAL-API-010: Pagination Total Count
The list response includes `total` field indicating total number of tasks across all pages.
Tool: curl
Evidence: Response status code 200, JSON body with `total` field as integer >= 0

### VAL-API-011: Pagination Pages Calculation
The list response includes `pages` field calculated as `ceil(total / limit)`.
Tool: curl
Evidence: Response status code 200, JSON body with `pages` field, verify `pages == ceil(total/limit)`

### VAL-API-012: Get Task Details Successfully
When GET request is made with a valid task ID, the system returns HTTP 200 with task details including model, dataset, status, created_at, and configuration.
Tool: curl
Evidence: Response status code 200, JSON body with `id`, `model`, `dataset`, `status`, `created_at` fields

### VAL-API-013: Task Not Found
When GET request is made with a non-existent task ID, the system returns HTTP 404 Not Found.
Tool: curl
Evidence: Response status code 404, JSON body with `error` field indicating task not found

### VAL-API-014: Get Status Successfully
When GET request is made with a valid task ID, the system returns HTTP 200 with status information including current state and progress.
Tool: curl
Evidence: Response status code 200, JSON body with `status`, `progress` fields

### VAL-API-015: Status Pending State
For a newly created task, status endpoint returns `status: "pending"` and `progress: 0`.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "pending"`, `progress: 0`

### VAL-API-016: Status Running State
For a task currently being processed, status endpoint returns `status: "running"` and `progress` between 1-99.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "running"`, `progress: 0-100`

### VAL-API-017: Status Completed State
For a completed task, status endpoint returns `status: "completed"` and `progress: 100`.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "completed"`, `progress: 100`

### VAL-API-018: Status Failed State
For a failed task, status endpoint returns `status: "failed"` and includes `error` field with failure reason.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "failed"`, `error` field

### VAL-API-019: Status Cancelled State
For a cancelled task, status endpoint returns `status: "cancelled"` and includes `cancelled_at` timestamp.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "cancelled"`, `cancelled_at` field

### VAL-API-020: Get Results Successfully
When GET request is made for a completed task, the system returns HTTP 200 with full evaluation results.
Tool: curl
Evidence: Response status code 200, JSON body with `results` object containing metrics

### VAL-API-021: Results Not Ready
When GET request is made for a task that is not yet completed (pending, running), the system returns HTTP 409 Conflict or HTTP 425 Too Early.
Tool: curl
Evidence: Response status code 409 or 425, JSON body with `error` field indicating results not ready

### VAL-API-022: Results Include Accuracy Metric
Evaluation results include overall accuracy score as a percentage or decimal.
Tool: curl
Evidence: Response status code 200, JSON body with `results.accuracy` or `results.overall_score` field

### VAL-API-023: Cancel Running Task Successfully
When DELETE request is made for a running or pending task, the system cancels it and returns HTTP 200 or 204.
Tool: curl
Evidence: Response status code 200 or 204, subsequent status check shows `status: "cancelled"`

### VAL-API-024: Cancel Already Completed Task
When DELETE request is made for an already completed task, the system returns HTTP 409 Conflict indicating task cannot be cancelled.
Tool: curl
Evidence: Response status code 409, JSON body with `error` field indicating task already completed

### VAL-API-025: List Models Successfully
When GET request is made, the system returns HTTP 200 with array of supported models including OpenAI GPT-4, Claude, and Qwen.
Tool: curl
Evidence: Response status code 200, JSON body with `models` array containing at least "gpt-4", "claude", "qwen"

### VAL-API-026: List Datasets Successfully
When GET request is made, the system returns HTTP 200 with array of supported datasets including MMLU, HellaSwag.
Tool: curl
Evidence: Response status code 200, JSON body with `datasets` array containing at least "MMLU", "HellaSwag"

### VAL-API-027: Health Check Returns 200
When GET request is made to /health, the system returns HTTP 200 indicating the service is running.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "healthy"` or `status: "ok"`

### VAL-API-028: Readiness Check Returns 200 When Healthy
When GET request is made to /ready and all dependencies (DB, Redis) are connected, the system returns HTTP 200.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "ready"` and dependency statuses

### VAL-API-029: Readiness Check Returns 503 When Unhealthy
When GET request is made to /ready and any dependency (DB or Redis) is unavailable, the system returns HTTP 503 Service Unavailable.
Tool: curl
Evidence: Response status code 503, JSON body with `status: "not_ready"` and failed dependency info

---

## Area: Kubernetes Job Integration

### VAL-K8S-001: Job Created with Correct Namespace
Job is created in the designated evaluation namespace and not in default namespace.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.metadata.namespace}'`

### VAL-K8S-002: Job Contains Required Labels
Job has required labels for identification: `app=llm-eval`, `eval-id=<eval-id>`, `model=<model-name>`.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.metadata.labels}'`

### VAL-K8S-003: Job Uses Correct Container Image
Job spec references the configured evaluation container image with correct tag.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].image}'`

### VAL-K8S-004: Job Has Resource Limits Defined
Job container has CPU and memory requests/limits specified.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].resources}'`

### VAL-K8S-005: Job Mounts ConfigMap Volume
Job spec includes volume mount for ConfigMap containing evaluation configuration.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.volumes}'`

### VAL-K8S-006: Job Mounts Secret Volume
Job spec includes volume mount for Secret containing API keys.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].volumeMounts}'`

### VAL-K8S-007: Job Sets Correct RestartPolicy
Job template has `RestartPolicy: OnFailure` or `Never` (not `Always`).
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.template.spec.restartPolicy}'`

### VAL-K8S-008: Job Sets Backoff Limit
Job has backoff limit configured to control retry attempts on failure.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.spec.backoffLimit}'`

### VAL-K8S-009: ConfigMap Created with Valid MMEngine Format
ConfigMap data contains valid MMEngine configuration format (Python config file).
Tool: kubectl
Evidence: `kubectl get configmap <configmap-name> -n <namespace> -o jsonpath='{.data}'`

### VAL-K8S-010: ConfigMap Contains Model Configuration
ConfigMap includes model configuration with correct model type, path, and API settings.
Tool: kubectl
Evidence: `kubectl get configmap <configmap-name> -n <namespace> -o jsonpath='{.data.config\.py}'`

### VAL-K8S-011: Secret Created for API Keys
Secret is created containing API keys for OpenAI, Claude, and Qwen.
Tool: kubectl
Evidence: `kubectl get secret <secret-name> -n <namespace> -o jsonpath='{.data}'`

### VAL-K8S-012: Secret Not Exposed in Pod Logs
API keys are not visible in pod logs or stdout.
Tool: kubectl
Evidence: `kubectl logs <pod-name> -n <namespace> | grep -i "api-key\|sk-\|key=" || echo "No key exposure found"`

### VAL-K8S-013: Job Status Correctly Reflects Running State
When Job Pod is running, API returns status "running" with progress information.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.active}'`

### VAL-K8S-014: Job Status Correctly Reflects Completed State
When Job completes successfully, API returns status "completed" and `.status.succeeded` is set.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.succeeded}'`

### VAL-K8S-015: Job Events Captured for Monitoring
Job events (Started, Completed, Failed) are captured and stored.
Tool: kubectl
Evidence: `kubectl get events --field-selector involvedObject.name=<job-name> -n <namespace>`

### VAL-K8S-016: Job Failure Triggers Retry Logic
Failed Job triggers retry with exponential backoff up to max retries.
Tool: kubectl
Evidence: `kubectl get job <job-name> -n <namespace> -o jsonpath='{.status.failed}'` and retry job creation logs

### VAL-K8S-017: OOM Killed Pods Detected and Reported
Out-of-memory killed pods are detected and reported as resource exhaustion errors.
Tool: kubectl
Evidence: `kubectl describe pod <pod-name> -n <namespace> | grep -i OOMKilled`

---

## Area: PostgreSQL Database

### VAL-DB-001: Evaluations Table Schema
The `evaluations` table exists with columns: id (UUID/serial), model_id, dataset_ids, config (JSONB), status (varchar/enum), created_at (timestamp), updated_at (timestamp), started_at (timestamp), completed_at (timestamp).
Tool: psql
Evidence: `\d evaluations` shows all columns with correct types and nullable flags

### VAL-DB-002: Models Table Schema
The `models` table exists with columns: id, name (NOT NULL), type (CHECK for 'api'/'local'), provider (CHECK for 'openai'/'claude'/'qwen'), api_key_ref.
Tool: psql
Evidence: `\d models` shows columns, constraints including CHECK constraints and unique index on name

### VAL-DB-003: Results Table Schema
The `results` table exists with columns: evaluation_id, accuracy (decimal), metrics (JSONB), summary (text). evaluation_id is foreign key referencing evaluations(id).
Tool: psql
Evidence: `\d results` shows columns and foreign key constraint to evaluations

### VAL-DB-004: Predictions Table Schema
The `predictions` table exists with columns: evaluation_id, question (NOT NULL), prediction (NOT NULL), answer (NOT NULL), correct (NOT NULL). evaluation_id references evaluations(id).
Tool: psql
Evidence: `\d predictions` shows columns with NOT NULL constraints and foreign key

### VAL-DB-005: Create Evaluation
INSERT into evaluations with valid model_id and dataset_ids succeeds. Status defaults to 'pending' and created_at is auto-populated.
Tool: psql
Evidence: Execute `INSERT INTO evaluations (...) RETURNING *;` returns new row with generated id and created_at

### VAL-DB-006: Update Evaluation Status
UPDATE evaluation status from 'pending' to 'running' sets started_at timestamp. UPDATE from 'running' to 'completed' sets completed_at timestamp.
Tool: psql
Evidence: Execute status transitions and verify started_at, completed_at, updated_at are set correctly

### VAL-DB-007: Insert Results with JSONB Metrics
INSERT into results with evaluation_id, accuracy, and metrics as JSONB object succeeds.
Tool: psql
Evidence: `INSERT INTO results (...) VALUES (..., '{"precision": 0.85}') RETURNING metrics->>'precision';` returns '0.85'

### VAL-DB-008: Batch Insert Predictions
INSERT multiple predictions for an evaluation succeeds. All rows are created with correct evaluation_id association.
Tool: psql
Evidence: `INSERT INTO predictions (...) VALUES ..., (...) RETURNING count(*);` returns inserted count

### VAL-DB-009: Paginated Evaluation List
SELECT evaluations with LIMIT and OFFSET returns correct page ordered by created_at DESC.
Tool: psql
Evidence: `SELECT * FROM evaluations ORDER BY created_at DESC LIMIT 10 OFFSET 0;` returns first 10 evaluations

### VAL-DB-010: Filter Evaluations by Status
SELECT evaluations WHERE status = 'completed' returns only completed evaluations.
Tool: psql
Evidence: `SELECT * FROM evaluations WHERE status = 'completed';` returns only completed records

### VAL-DB-011: Foreign Key Enforcement - Results
INSERT into results with non-existent evaluation_id fails with foreign key violation error.
Tool: psql
Evidence: `INSERT INTO results (evaluation_id, accuracy) VALUES ('non-existent-uuid', 0.5);` returns ERROR: foreign key violation

### VAL-DB-012: Query JSONB Metrics Fields
SELECT results where metrics JSONB contains specific key-value pair.
Tool: psql
Evidence: `SELECT * FROM results WHERE metrics->>'f1' = '0.87';` returns results with matching f1 score

### VAL-DB-013: Index on Evaluations Status
EXPLAIN ANALYZE on SELECT evaluations WHERE status = 'completed' shows index scan.
Tool: psql
Evidence: `EXPLAIN ANALYZE SELECT * FROM evaluations WHERE status = 'completed';` output shows "Index Scan"

### VAL-DB-014: Connection Pool Max Connections
Application respects max connection pool limit (25 connections).
Tool: psql
Evidence: Query `SELECT count(*) FROM pg_stat_activity WHERE datname = 'evaluations';` shows bounded connections

---

## Area: OpenCompass CLI Integration

### VAL-OC-001: CLI Command Construction with Required Arguments
The system constructs CLI command with correct Python interpreter and run.py entry point, including required arguments (--datasets, --hf-path, --work-dir).
Tool: subprocess
Evidence: Command string logged before execution; full command line with arguments

### VAL-OC-002: Environment Variables for API Keys
The system injects API keys (OPENAI_API_KEY, ANTHROPIC_API_KEY, DASHSCOPE_API_KEY) as environment variables into the subprocess.
Tool: subprocess
Evidence: Environment dictionary (with keys redacted); successful API authentication in output

### VAL-OC-003: Subprocess Timeout Handling
The system enforces configurable timeout on subprocess execution, terminating process if evaluation exceeds threshold.
Tool: subprocess
Evidence: Timeout exception logs; process termination recorded; error message returned

### VAL-OC-004: Valid Python Configuration File Syntax
The system generates MMEngine-compatible Python configuration files with valid syntax.
Tool: subprocess
Evidence: Generated config file content; successful config load in logs

### VAL-OC-005: Model Configuration Required Fields
Generated model config includes required fields: type, path, max_seq_len, max_out_len, batch_size, run_cfg.
Tool: subprocess
Evidence: Generated config file; field validation output

### VAL-OC-006: OpenAI Model Configuration
Generated OpenAI model config has type=OpenAI, path='gpt-4', key from OPENAI_API_KEY env var, num_gpus=0.
Tool: subprocess
Evidence: Generated OpenAI model config section; successful API call in logs

### VAL-OC-007: Claude Model Configuration
Generated Claude model config has correct type, key from ANTHROPIC_API_KEY env var.
Tool: subprocess
Evidence: Generated Claude model config; successful Claude API call logs

### VAL-OC-008: Qwen Model Configuration
Generated Qwen model config uses DashScope API, key from DASHSCOPE_API_KEY env var.
Tool: subprocess
Evidence: Generated Qwen model config; successful Qwen API call logs

### VAL-OC-009: JSON Predictions Parsing
The system parses JSON prediction files from predictions/ directory, extracting question, prediction, answer fields.
Tool: subprocess
Evidence: Parsed prediction data structure; sample count matches expected

### VAL-OC-010: CSV Summary Extraction
The system extracts summary CSV from output directory, parsing dataset names, model names, accuracy scores.
Tool: subprocess
Evidence: Parsed CSV content; extracted scores dictionary

### VAL-OC-011: Timestamp-Based Output Directory Naming
Output directories created with timestamp-based naming (YYYYMMDD_HHMMSS format).
Tool: subprocess
Evidence: Output directory path; creation timestamp logged

### VAL-OC-012: Output Directory Cleanup After Collection
Output directory cleaned up after successful result collection when cleanup is enabled.
Tool: subprocess
Evidence: Directory existence check after cleanup; cleanup operation logged

### VAL-OC-013: Non-Zero Exit Code Capture
Subprocess non-zero exit codes captured and reported with error details.
Tool: subprocess
Evidence: Exit code in result dictionary; stderr captured; error message returned

---

## Cross-Area Flows

### VAL-CROSS-001: Full Evaluation Lifecycle End-to-End
When user creates evaluation via API, the system orchestrates: API creates task → K8s Job spawns → OpenCompass executes → Results stored in DB → API returns results. Status transitions pending → running → completed.
Tool: curl + kubectl + psql
Evidence: API POST returns 202 with task_id; kubectl shows Job created; DB shows completed status with predictions; API returns paginated results

### VAL-CROSS-002: Evaluation Status Reflects Real Job State
When K8s Job transitions Running → Completed, API status reflects same state, DB updated_at timestamp changes.
Tool: kubectl + curl + psql
Evidence: kubectl shows "Complete"; API status shows "completed"; DB shows recent updated_at

### VAL-CROSS-003: Results Persist Correctly After Job Completion
When OpenCompass writes results, DB predictions inserted with correct evaluation_id, API returns paginated results matching stored data.
Tool: curl + psql + kubectl logs
Evidence: DB predictions count matches expected; API returns correct predictions with evaluation_id

### VAL-CROSS-004: Cancellation Terminates Job and Updates Status
When user cancels running evaluation via DELETE, K8s Job terminated, DB status updated to 'cancelled', partial results preserved.
Tool: curl + kubectl + psql
Evidence: DELETE returns 200; kubectl shows Job deleted/not found; DB shows "cancelled" status

### VAL-CROSS-005: OpenCompass Failure Propagates to API and DB
When OpenCompass fails, K8s Job marked failed, DB records error message, API returns 'failed' status.
Tool: kubectl + psql + curl
Evidence: kubectl shows failed Job; DB shows failed with error_info; API status returns "failed" with error

### VAL-CROSS-006: Model API Key Securely Mounted to Job
API key created in K8s Secret, mounted as env var to Job pod, not logged or exposed in API/DB.
Tool: kubectl + curl + psql
Evidence: kubectl shows secret exists; pod env has key; logs don't contain key; API/DB don't return key

### VAL-CROSS-007: Concurrent Evaluations Isolated Correctly
When 5+ evaluations run simultaneously, each has isolated K8s Job, DB records, result storage, no cross-contamination.
Tool: curl + kubectl + psql
Evidence: kubectl shows 5 distinct jobs; DB shows 5 distinct evaluations; predictions have no overlap

### VAL-CROSS-008: Large Result Set Paginated Correctly
When evaluation produces 10,000+ predictions, API paginates correctly, DB query uses efficient OFFSET/LIMIT.
Tool: curl + psql
Evidence: API page=1 returns 100 items; DB EXPLAIN ANALYZE shows efficient query

### VAL-CROSS-009: Status Update Race Condition Prevention
When Job completion and user cancellation occur simultaneously, final status deterministic (completed or cancelled, not ambiguous).
Tool: curl + kubectl + psql
Evidence: DB shows exactly one terminal status; kubectl shows deterministic final state

### VAL-CROSS-010: High Concurrency Load Handling
When 20 concurrent evaluations created within 1 second, all accepted by API, 20 K8s Jobs created, all complete successfully.
Tool: curl + kubectl + psql
Evidence: All API requests return 202; kubectl shows 20 jobs; DB shows 20 completed evaluations

---

## Summary

| Area | Assertion Count |
|------|-----------------|
| API Endpoints | 29 |
| Kubernetes Job Integration | 17 |
| PostgreSQL Database | 14 |
| OpenCompass CLI Integration | 13 |
| Cross-Area Flows | 10 |
| **Total** | **83** |
