# API Behavioral Assertions for LLM Evaluation Backend

This document enumerates all behavioral assertions for the Go-based LLM evaluation backend system.

---

## POST /api/v1/evaluations - Create Evaluation Task

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

### VAL-API-008: Optional Parameters Accepted
When POST request includes optional parameters (e.g., `batch_size`, `temperature`), the system accepts and applies them within valid ranges.
Tool: curl
Evidence: Response status code 202, subsequent GET on task shows applied parameters

### VAL-API-009: Invalid Optional Parameter Range
When POST request includes optional parameter outside valid range (e.g., `temperature: -0.5`), the system returns HTTP 400 Bad Request with constraint violation message.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field describing valid range

### VAL-API-010: Malformed JSON Body
When POST request contains invalid JSON in request body, the system returns HTTP 400 Bad Request with parse error message.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field containing JSON parse error

### VAL-API-011: Content-Type Header Required
When POST request is made without `Content-Type: application/json` header, the system returns HTTP 415 Unsupported Media Type or HTTP 400 Bad Request.
Tool: curl
Evidence: Response status code 415 or 400, appropriate error message

---

## GET /api/v1/evaluations - List Evaluation Tasks

### VAL-API-012: List Tasks Default Pagination
When GET request is made without pagination parameters, the system returns default page (page=1) with default limit (e.g., 10 items).
Tool: curl
Evidence: Response status code 200, JSON body with `tasks` array, `page: 1`, `limit: 10`

### VAL-API-013: List Tasks Custom Pagination
When GET request includes `?page=2&limit=5`, the system returns the second page with 5 items per page.
Tool: curl
Evidence: Response status code 200, JSON body with `page: 2`, `limit: 5`, `tasks` array of max 5 items

### VAL-API-014: Pagination Total Count
The list response includes `total` field indicating total number of tasks across all pages.
Tool: curl
Evidence: Response status code 200, JSON body with `total` field as integer >= 0

### VAL-API-015: Pagination Pages Calculation
The list response includes `pages` field calculated as `ceil(total / limit)`.
Tool: curl
Evidence: Response status code 200, JSON body with `pages` field, verify `pages == ceil(total/limit)`

### VAL-API-016: Empty List Response
When no tasks exist, the system returns HTTP 200 with empty `tasks` array and `total: 0`.
Tool: curl
Evidence: Response status code 200, JSON body with `tasks: []`, `total: 0`, `pages: 0`

### VAL-API-017: Invalid Page Parameter
When GET request includes invalid `page` parameter (e.g., `page=-1` or `page=abc`), the system returns HTTP 400 Bad Request.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field describing valid page format

### VAL-API-018: Invalid Limit Parameter
When GET request includes invalid `limit` parameter (e.g., `limit=0` or `limit=10000`), the system returns HTTP 400 Bad Request or clamps to valid range.
Tool: curl
Evidence: Response status code 400 or 200 with clamped limit, appropriate error or adjusted values

### VAL-API-019: Page Beyond Available Pages
When GET request requests a page beyond available data (e.g., `page=999`), the system returns HTTP 200 with empty `tasks` array.
Tool: curl
Evidence: Response status code 200, JSON body with `tasks: []`

---

## GET /api/v1/evaluations/:id - Get Task Details

### VAL-API-020: Get Task Details Successfully
When GET request is made with a valid task ID, the system returns HTTP 200 with task details including model, dataset, status, created_at, and configuration.
Tool: curl
Evidence: Response status code 200, JSON body with `id`, `model`, `dataset`, `status`, `created_at` fields

### VAL-API-021: Task Not Found
When GET request is made with a non-existent task ID, the system returns HTTP 404 Not Found.
Tool: curl
Evidence: Response status code 404, JSON body with `error` field indicating task not found

### VAL-API-022: Invalid Task ID Format
When GET request is made with an invalid task ID format (not UUID), the system returns HTTP 400 Bad Request.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field indicating invalid ID format

### VAL-API-023: Task Details Include Timestamps
Task details response includes `created_at` timestamp in ISO 8601 format and optionally `updated_at` if task has progressed.
Tool: curl
Evidence: Response status code 200, JSON body with `created_at` matching ISO 8601 format

### VAL-API-024: Task Details Include Configuration
Task details response includes the original request configuration (model, dataset, parameters).
Tool: curl
Evidence: Response status code 200, JSON body with `config` or `parameters` object

---

## GET /api/v1/evaluations/:id/status - Get Task Status

### VAL-API-025: Get Status Successfully
When GET request is made with a valid task ID, the system returns HTTP 200 with status information including current state and progress.
Tool: curl
Evidence: Response status code 200, JSON body with `status`, `progress` fields

### VAL-API-026: Status Not Found
When GET request is made with a non-existent task ID, the system returns HTTP 404 Not Found.
Tool: curl
Evidence: Response status code 404, JSON body with `error` field indicating task not found

### VAL-API-027: Status Pending State
For a newly created task, status endpoint returns `status: "pending"` and `progress: 0`.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "pending"`, `progress: 0`

### VAL-API-028: Status Running State
For a task currently being processed, status endpoint returns `status: "running"` and `progress` between 1-99.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "running"`, `progress: 0-100`

### VAL-API-029: Status Completed State
For a completed task, status endpoint returns `status: "completed"` and `progress: 100`.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "completed"`, `progress: 100`

### VAL-API-030: Status Failed State
For a failed task, status endpoint returns `status: "failed"` and includes `error` field with failure reason.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "failed"`, `error` field

### VAL-API-031: Status Cancelled State
For a cancelled task, status endpoint returns `status: "cancelled"` and includes `cancelled_at` timestamp.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "cancelled"`, `cancelled_at` field

### VAL-API-032: Progress Percentage Format
The `progress` field is an integer or float between 0 and 100 representing percentage completion.
Tool: curl
Evidence: Response status code 200, JSON body with `progress` as number between 0-100

---

## GET /api/v1/evaluations/:id/results - Get Evaluation Results

### VAL-API-033: Get Results Successfully
When GET request is made for a completed task, the system returns HTTP 200 with full evaluation results.
Tool: curl
Evidence: Response status code 200, JSON body with `results` object containing metrics

### VAL-API-034: Results Not Found
When GET request is made with a non-existent task ID, the system returns HTTP 404 Not Found.
Tool: curl
Evidence: Response status code 404, JSON body with `error` field indicating task not found

### VAL-API-035: Results Not Ready
When GET request is made for a task that is not yet completed (pending, running), the system returns HTTP 409 Conflict or HTTP 425 Too Early.
Tool: curl
Evidence: Response status code 409 or 425, JSON body with `error` field indicating results not ready

### VAL-API-036: Results Include Accuracy Metric
Evaluation results include overall accuracy score as a percentage or decimal.
Tool: curl
Evidence: Response status code 200, JSON body with `results.accuracy` or `results.overall_score` field

### VAL-API-037: Results Include Per-Category Breakdown
Evaluation results include breakdown by category (if applicable to dataset).
Tool: curl
Evidence: Response status code 200, JSON body with `results.categories` or `results.breakdown` array

### VAL-API-038: Results Include Sample Count
Evaluation results include the number of samples evaluated.
Tool: curl
Evidence: Response status code 200, JSON body with `results.samples_evaluated` or `results.total_samples` field

### VAL-API-039: Results for Failed Task
When GET request is made for a failed task, the system returns HTTP 200 with partial results if any, or HTTP 409 Conflict with failure details.
Tool: curl
Evidence: Response status code 200 or 409, JSON body with partial results or error explanation

---

## DELETE /api/v1/evaluations/:id - Cancel/Delete Task

### VAL-API-040: Cancel Running Task Successfully
When DELETE request is made for a running or pending task, the system cancels it and returns HTTP 200 or 204.
Tool: curl
Evidence: Response status code 200 or 204, subsequent status check shows `status: "cancelled"`

### VAL-API-041: Cancel Not Found
When DELETE request is made with a non-existent task ID, the system returns HTTP 404 Not Found.
Tool: curl
Evidence: Response status code 404, JSON body with `error` field indicating task not found

### VAL-API-042: Cancel Already Completed Task
When DELETE request is made for an already completed task, the system returns HTTP 409 Conflict indicating task cannot be cancelled.
Tool: curl
Evidence: Response status code 409, JSON body with `error` field indicating task already completed

### VAL-API-043: Cancel Already Cancelled Task
When DELETE request is made for an already cancelled task, the system returns HTTP 409 Conflict or HTTP 200 with idempotent response.
Tool: curl
Evidence: Response status code 409 or 200, consistent with idempotent DELETE semantics

### VAL-API-044: Cancel Invalid Task ID
When DELETE request is made with an invalid task ID format, the system returns HTTP 400 Bad Request.
Tool: curl
Evidence: Response status code 400, JSON body with `error` field indicating invalid ID format

### VAL-API-045: Cancel Idempotency
Multiple DELETE requests to the same pending task should be idempotent - subsequent DELETE requests return same result.
Tool: curl
Evidence: Response status code 200 or 409 consistently for repeated requests

---

## GET /api/v1/models - List Supported Models

### VAL-API-046: List Models Successfully
When GET request is made, the system returns HTTP 200 with array of supported models including OpenAI GPT-4, Claude, and Qwen.
Tool: curl
Evidence: Response status code 200, JSON body with `models` array containing at least "gpt-4", "claude", "qwen"

### VAL-API-047: Models Include Display Names
Each model entry includes both `id` and `display_name` or `name` field.
Tool: curl
Evidence: Response status code 200, JSON body with each model having `id` and `name` or `display_name` fields

### VAL-API-048: Models Include Version Info
Each model entry optionally includes version information or variant details.
Tool: curl
Evidence: Response status code 200, JSON body with model entries containing `version` or `variants` fields

### VAL-API-049: Models Response Is Cacheable
The models list endpoint returns appropriate cache headers since models change infrequently.
Tool: curl
Evidence: Response headers include `Cache-Control`, `ETag`, or `Last-Modified` header

---

## GET /api/v1/datasets - List Supported Datasets

### VAL-API-050: List Datasets Successfully
When GET request is made, the system returns HTTP 200 with array of supported datasets including MMLU, HellaSwag.
Tool: curl
Evidence: Response status code 200, JSON body with `datasets` array containing at least "MMLU", "HellaSwag"

### VAL-API-051: Datasets Include Descriptions
Each dataset entry includes `id`, `name`, and `description` fields.
Tool: curl
Evidence: Response status code 200, JSON body with each dataset having `id`, `name`, `description` fields

### VAL-API-052: Datasets Include Sample Counts
Each dataset entry includes the number of samples/questions it contains.
Tool: curl
Evidence: Response status code 200, JSON body with each dataset having `sample_count` or `total_samples` field

### VAL-API-053: Datasets Include Categories
Each dataset entry optionally includes category or subject breakdown (if applicable).
Tool: curl
Evidence: Response status code 200, JSON body with dataset entries containing `categories` or `subjects` array

### VAL-API-054: Datasets Response Is Cacheable
The datasets list endpoint returns appropriate cache headers since datasets change infrequently.
Tool: curl
Evidence: Response headers include `Cache-Control`, `ETag`, or `Last-Modified` header

---

## GET /health - Health Check (Liveness)

### VAL-API-055: Health Check Returns 200
When GET request is made to /health, the system returns HTTP 200 indicating the service is running.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "healthy"` or `status: "ok"`

### VAL-API-056: Health Check Minimal Response
Health check returns minimal response without external dependency checks (liveness probe).
Tool: curl
Evidence: Response status code 200, JSON body contains only basic status, no DB/Redis checks

### VAL-API-057: Health Check Response Time
Health check responds within acceptable latency threshold (e.g., < 100ms) for kubelet probes.
Tool: curl
Evidence: Response time measured via curl timing flags, should be < 100ms

---

## GET /ready - Readiness Check (DB + Redis Connectivity)

### VAL-API-058: Readiness Check Returns 200 When Healthy
When GET request is made to /ready and all dependencies (DB, Redis) are connected, the system returns HTTP 200.
Tool: curl
Evidence: Response status code 200, JSON body with `status: "ready"` and dependency statuses

### VAL-API-059: Readiness Check Returns 503 When Unhealthy
When GET request is made to /ready and any dependency (DB or Redis) is unavailable, the system returns HTTP 503 Service Unavailable.
Tool: curl
Evidence: Response status code 503, JSON body with `status: "not_ready"` and failed dependency info

### VAL-API-060: Readiness Check Includes Dependency Status
Readiness response includes individual status for each dependency (database, redis).
Tool: curl
Evidence: Response status code 200, JSON body with `database: "connected"` and `redis: "connected"` fields

### VAL-API-061: Readiness Check Database Failure
When database connection fails, readiness check returns HTTP 503 with database error details.
Tool: curl
Evidence: Response status code 503, JSON body with `database: "disconnected"` or error message

### VAL-API-062: Readiness Check Redis Failure
When Redis connection fails, readiness check returns HTTP 503 with Redis error details.
Tool: curl
Evidence: Response status code 503, JSON body with `redis: "disconnected"` or error message

### VAL-API-063: Readiness Check Partial Degradation
When one dependency is degraded but core functionality available, readiness may return HTTP 200 with degraded status or HTTP 503 based on configuration.
Tool: curl
Evidence: Response status code 200 or 503, JSON body indicates degraded state

---

## State Transitions

### VAL-API-064: State Transition Pending To Running
A task transitions from `pending` to `running` when picked up by a worker.
Tool: curl
Evidence: Poll status endpoint, observe transition from `status: "pending"` to `status: "running"`

### VAL-API-065: State Transition Running To Completed
A running task transitions to `completed` when evaluation finishes successfully.
Tool: curl
Evidence: Poll status endpoint, observe transition from `status: "running"` to `status: "completed"` with `progress: 100`

### VAL-API-066: State Transition Running To Failed
A running task transitions to `failed` when an error occurs during evaluation.
Tool: curl
Evidence: Poll status endpoint, observe transition from `status: "running"` to `status: "failed"` with error details

### VAL-API-067: State Transition Pending To Cancelled
A pending task can be cancelled and transitions to `cancelled` state.
Tool: curl
Evidence: DELETE pending task, subsequent GET shows `status: "cancelled"`

### VAL-API-068: State Transition Running To Cancelled
A running task can be cancelled and transitions to `cancelled` state.
Tool: curl
Evidence: DELETE running task, subsequent GET shows `status: "cancelled"`

### VAL-API-069: No Transition From Completed
A completed task cannot transition to any other state (terminal state).
Tool: curl
Evidence: DELETE completed task returns 409, status remains `completed`

### VAL-API-070: No Transition From Failed
A failed task cannot transition to any other state (terminal state).
Tool: curl
Evidence: DELETE failed task returns 409, status remains `failed`

---

## Error Handling

### VAL-API-071: Error Response Format
All error responses follow consistent JSON format with `error` field containing error message and optionally `code` field.
Tool: curl
Evidence: Error responses contain JSON body with `error` string field

### VAL-API-072: Error Response Includes Request ID
Error responses include `request_id` or `trace_id` for debugging purposes.
Tool: curl
Evidence: Error response headers or body include `X-Request-ID` or `request_id` field

### VAL-API-073: Internal Server Error Handling
When an unexpected error occurs, the system returns HTTP 500 with generic error message (no stack traces exposed).
Tool: curl
Evidence: Response status code 500, JSON body with generic `error` message, no stack traces

### VAL-API-074: Rate Limiting Response
When rate limit is exceeded, the system returns HTTP 429 Too Many Requests with `Retry-After` header.
Tool: curl
Evidence: Response status code 429, headers include `Retry-After`, body includes rate limit info

### VAL-API-075: Request Timeout Handling
When request processing times out, the system returns HTTP 504 Gateway Timeout or HTTP 408 Request Timeout.
Tool: curl
Evidence: Response status code 504 or 408 for long-running requests

---

## Concurrent Access

### VAL-API-076: Concurrent Status Polls
Multiple concurrent GET requests to the same task status endpoint return consistent results.
Tool: curl
Evidence: Multiple parallel curl requests return same status for same task ID

### VAL-API-077: Concurrent Cancel Attempts
Multiple concurrent DELETE requests to the same task result in only one cancellation being processed.
Tool: curl
Evidence: Parallel DELETE requests return 200/409 appropriately, task ends in `cancelled` state

### VAL-API-078: Read After Write Consistency
After creating a task (POST), immediately reading it (GET) returns the newly created task.
Tool: curl
Evidence: POST followed by immediate GET returns the created task details

---

## Summary

Total Assertions: 78
- POST /api/v1/evaluations: 11 assertions (VAL-API-001 to VAL-API-011)
- GET /api/v1/evaluations (list): 8 assertions (VAL-API-012 to VAL-API-019)
- GET /api/v1/evaluations/:id: 5 assertions (VAL-API-020 to VAL-API-024)
- GET /api/v1/evaluations/:id/status: 8 assertions (VAL-API-025 to VAL-API-032)
- GET /api/v1/evaluations/:id/results: 7 assertions (VAL-API-033 to VAL-API-039)
- DELETE /api/v1/evaluations/:id: 6 assertions (VAL-API-040 to VAL-API-045)
- GET /api/v1/models: 4 assertions (VAL-API-046 to VAL-API-049)
- GET /api/v1/datasets: 5 assertions (VAL-API-050 to VAL-API-054)
- GET /health: 3 assertions (VAL-API-055 to VAL-API-057)
- GET /ready: 6 assertions (VAL-API-058 to VAL-API-063)
- State Transitions: 7 assertions (VAL-API-064 to VAL-API-070)
- Error Handling: 5 assertions (VAL-API-071 to VAL-API-075)
- Concurrent Access: 3 assertions (VAL-API-076 to VAL-API-078)
