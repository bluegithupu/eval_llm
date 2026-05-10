# User Testing

Testing surface, required testing skills/tools, and resource cost classification for LLM evaluation backend.

**What belongs here:** Validation surface findings, required testing skills/tools, validation prerequisites, resource cost classification.

---

## Validation Surface

### API Endpoints (Primary Surface)

**Surface:** REST API at http://localhost:3100

**Endpoints:**
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/evaluations` | POST | Create evaluation task |
| `/api/v1/evaluations` | GET | List tasks (paginated) |
| `/api/v1/evaluations/:id` | GET | Get task details |
| `/api/v1/evaluations/:id/status` | GET | Get task status |
| `/api/v1/evaluations/:id/results` | GET | Get evaluation results |
| `/api/v1/evaluations/:id` | DELETE | Cancel task |
| `/api/v1/models` | GET | List supported models |
| `/api/v1/datasets` | GET | List supported datasets |
| `/health` | GET | Liveness probe |
| `/ready` | GET | Readiness probe |

**Testing Tool:** `curl` (CLI-based HTTP client)

**Test Categories:**
1. Request validation (missing fields, invalid values)
2. Response format (JSON structure, status codes)
3. Pagination behavior (page, limit, total, pages)
4. Status transitions (pending → running → completed)
5. Error handling (400, 404, 409, 500, 503)

### PostgreSQL Database (Secondary Surface)

**Surface:** Database at localhost:3105

**Tables:** evaluations, models, datasets, results, predictions, logs

**Testing Tool:** `psql` (CLI PostgreSQL client)

**Test Categories:**
1. Schema verification (columns, constraints, indexes)
2. CRUD operations (insert, update, query)
3. JSONB operations (metrics query, config update)
4. Index efficiency (EXPLAIN ANALYZE)
5. Connection pool (max connections)

### Redis Cache (Secondary Surface)

**Surface:** Redis at localhost:3106

**Key Patterns:** eval:status:{id}, eval:progress:{id}

**Testing Tool:** `redis-cli` (CLI Redis client)

**Test Categories:**
1. Status caching (GET/SET operations)
2. TTL verification (24-hour expiration)
3. Key cleanup on completion

### Kubernetes Jobs (Secondary Surface)

**Surface:** Kubernetes namespace `llm-eval`

**Resources:** Jobs, ConfigMaps, Secrets, Pods

**Testing Tool:** `kubectl` (CLI Kubernetes client)

**Test Categories:**
1. Job creation (labels, resources, volumes)
2. ConfigMap content (MMEngine format)
3. Secret management (base64 encoding, no exposure)
4. Job status (running, completed, failed)
5. Resource cleanup (Job deletion after completion)

---

## Validation Prerequisites

### Service Setup

| Prerequisite | Verification | Allowlist Required |
|--------------|--------------|-------------------|
| PostgreSQL container running | `pg_isready -h localhost -p 3105` | No |
| Redis container running | `redis-cli -p 3106 ping` | No |
| API server running | `curl -sf http://localhost:3100/health` | No |
| Kubernetes namespace exists | `kubectl get ns llm-eval` | No |
| OpenCompass container image | `kubectl apply -f job.yaml` | No |

### Credentials

| Credential | Required For | Setup |
|------------|--------------|-------|
| PostgreSQL password | DB connection | Set in Docker container env |
| OpenAI API key | GPT-4 evaluation | User provides, stored in K8s Secret |
| Anthropic API key | Claude evaluation | Optional, user provides |
| DashScope API key | Qwen evaluation | Optional, user provides |

---

## Validation Concurrency

### Resource Measurements

**Baseline System:**
- Memory: ~6 GB used at idle
- CPU: ~12 cores available
- Processes: Moderate load

**Validation Resource Usage:**

| Surface | Per-Validator Cost | Notes |
|---------|-------------------|-------|
| API (curl) | ~5 MB | Lightweight CLI tool |
| PostgreSQL (psql) | ~10 MB | CLI client, minimal overhead |
| Redis (redis-cli) | ~5 MB | CLI client, minimal overhead |
| Kubernetes (kubectl) | ~20 MB | CLI client, API calls |

**Total Per-Validator:** ~40 MB (all surfaces)

**Max Concurrent Validators:** 5

**Rationale:**
- Available headroom: ~12 GB memory * 0.7 = 8.4 GB
- Per-validator cost: 40 MB
- 5 validators = 200 MB (well within budget)
- API server and services add ~200-500 MB
- Total validation load: ~500 MB (safe)

**Concurrency Strategy:**
- Group related assertions by surface
- Run API endpoint tests in parallel (5 concurrent)
- Run DB/K8s tests sequentially (avoid resource conflicts)
- Cross-area flows run sequentially (complex dependencies)

---

## Test Execution Strategy

### Per-Surface Execution

**API Endpoints:**
1. Start with health/ready checks (confirm services up)
2. Test model/dataset list endpoints (static data)
3. Test evaluation CRUD (create, list, get, status, results, cancel)
4. Test error scenarios (validation, not found, conflict)
5. Test pagination edge cases

**Database:**
1. Schema verification (one pass)
2. CRUD tests (one pass per table)
3. Query tests (pagination, filtering)
4. JSONB tests (metrics, config)
5. Index efficiency tests

**Redis:**
1. Status caching tests
2. TTL tests
3. Cleanup tests

**Kubernetes:**
1. Job creation tests (sequential to avoid name conflicts)
2. ConfigMap/Secret tests
3. Status monitoring tests
4. Cleanup tests

### Cross-Area Flows

Execute sequentially (complex dependencies):
1. Full lifecycle (create → Job → complete → results)
2. Cancellation flow
3. Error propagation
4. Concurrency isolation
5. Secret security verification

---

## Testing Skills/Tools Required

| Tool | Installation | Purpose |
|------|--------------|---------|
| `curl` | Built-in / brew install curl | API endpoint testing |
| `psql` | brew install postgresql | Database testing |
| `redis-cli` | brew install redis | Cache testing |
| `kubectl` | brew install kubectl | Kubernetes testing |
| `jq` | brew install jq | JSON parsing (optional) |

---

## Flow Validator Guidance: API Endpoints

**Surface:** REST API at http://localhost:3100

**Testing Tool:** `curl` (CLI HTTP client)

**Isolation Rules:**
- All assertions are read-only tests of health/readiness endpoints
- No shared state mutation - validators can run concurrently
- Each validator tests independent endpoint behavior

**Assertions Covered:**
- VAL-API-027: Health check returns 200
- VAL-API-028: Readiness check returns 200 when healthy
- VAL-API-029: Readiness check returns 503 when unhealthy

**Testing Instructions:**
1. Use curl to hit each endpoint
2. Verify status codes match expectations
3. Verify response body contains expected fields

**Boundaries:**
- URL: http://localhost:3100
- Endpoints: /health, /ready
- Do not modify any data, only read endpoints

---

## Flow Validator Guidance: Kubernetes Jobs

**Surface:** Kubernetes cluster with namespace `llm-eval`

**Testing Tool:** `kubectl` (CLI Kubernetes client)

**Prerequisites:**
- kubectl configured with access to a running Kubernetes cluster
- Namespace `llm-eval` exists (create with `kubectl create ns llm-eval` if needed)
- Kind cluster can be created with `kind create cluster --name my-cluster`

**Isolation Rules:**
- Job creation uses unique names (evaluation ID-based) to avoid conflicts
- Each assertion tests independent Job properties (labels, resources, volumes)
- ConfigMap and Secret tests may have dependencies on Job creation
- Run Job creation tests sequentially to avoid naming conflicts
- Use `kubectl delete job <name> -n llm-eval` to clean up after tests

**Assertions Covered:**
- VAL-K8S-001: Job created in correct namespace
- VAL-K8S-002: Job contains required labels
- VAL-K8S-003: Job uses correct container image
- VAL-K8S-004: Job has resource limits defined
- VAL-K8S-005: Job mounts ConfigMap volume
- VAL-K8S-006: Job mounts Secret volume
- VAL-K8S-007: Job sets correct RestartPolicy
- VAL-K8S-008: Job sets backoff limit
- VAL-K8S-009: ConfigMap created with valid MMEngine format
- VAL-K8S-010: ConfigMap contains model configuration
- VAL-K8S-011: Secret created for API keys
- VAL-K8S-012: Secret not exposed in pod logs
- VAL-K8S-013: Job status correctly reflects running state
- VAL-K8S-014: Job status correctly reflects completed state
- VAL-K8S-015: Job events captured for monitoring
- VAL-K8S-016: Job failure triggers retry logic
- VAL-K8S-017: OOM killed pods detected and reported

**Testing Instructions:**
1. Verify kubectl can connect to cluster: `kubectl cluster-info`
2. Create namespace if needed: `kubectl create ns llm-eval`
3. For Job creation tests: Create a test Job using kubectl or via API
4. Verify Job properties using `kubectl get job -n llm-eval -o yaml`
5. Verify ConfigMap: `kubectl get configmap -n llm-eval -o yaml`
6. Verify Secret: `kubectl get secret -n llm-eval -o yaml` (base64 encoded)
7. Check pod logs: `kubectl logs job/<name> -n llm-eval`
8. Check events: `kubectl get events -n llm-eval --field-selector involvedObject.name=<job-name>`

**Boundaries:**
- Namespace: llm-eval
- Context: kind-my-cluster (or other configured context)
- Do not modify other namespaces or resources outside llm-eval
- Clean up test Jobs after verification

---

## Flow Validator Guidance: PostgreSQL Database

**Surface:** PostgreSQL database via Docker container `eval-postgres`

**Testing Tool:** `docker exec eval-postgres psql -U eval_user -d evaluations`

**Isolation Rules:**
- Schema verification assertions (VAL-DB-001 to VAL-DB-004) are read-only
- CRUD assertions (VAL-DB-005 to VAL-DB-012) modify shared tables
- Run CRUD tests sequentially within single validator to avoid conflicts
- Use test-specific data (e.g., test UUIDs) for INSERT operations
- Clean up test data after assertions if possible

**Assertions Covered:**
- VAL-DB-001: Evaluations table schema
- VAL-DB-002: Models table schema
- VAL-DB-003: Results table schema
- VAL-DB-004: Predictions table schema
- VAL-DB-005: Create evaluation
- VAL-DB-006: Update evaluation status
- VAL-DB-007: Insert results with JSONB metrics
- VAL-DB-008: Batch insert predictions
- VAL-DB-009: Paginated evaluation list
- VAL-DB-010: Filter evaluations by status
- VAL-DB-011: Foreign key enforcement - Results
- VAL-DB-012: Query JSONB metrics fields
- VAL-DB-013: Index on evaluations status
- VAL-DB-014: Connection pool max connections

**Testing Instructions:**
1. Use docker exec to run psql commands inside container
2. Schema tests: Use `\d <table>` to verify columns and constraints
3. CRUD tests: Execute INSERT/UPDATE/SELECT statements
4. Use test UUIDs (e.g., 'test-00000000-0000-0000-0000-000000000001')
5. Verify JSONB operations work correctly
6. Use EXPLAIN ANALYZE to verify index usage

**Boundaries:**
- Container: eval-postgres
- Database: evaluations
- User: eval_user
- Tables: evaluations, models, datasets, results, predictions
- Do not modify production data, use test-specific entries

---

*User testing guidance for LLM evaluation backend.*
