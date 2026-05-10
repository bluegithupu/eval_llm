# AGENTS.md: Worker Guidance for LLM Evaluation Backend

This document provides operational guidance for workers implementing the LLM evaluation backend.

---

## Mission Boundaries (NEVER VIOLATE)

**Port Range:**
- API server: 3100 ONLY
- PostgreSQL: 3105 ONLY
- Redis: 3106 ONLY
- Never start services on other ports (avoid 3000, 5000, 8080, 5432, 6379 conflicts)

**External Services:**
- PostgreSQL on localhost:3105 (created via Docker for this mission)
- Redis on localhost:3106 (created via Docker for this mission)
- Kubernetes cluster via kubectl (use existing context)

**Off-Limits:**
- Port 8080 (used by existing python process)
- Port 7000, 5000 (ControlCenter)
- Port 33331 (clash-verge)
- Default PostgreSQL port 5432
- Default Redis port 6379

**Resources:**
- PostgreSQL connection pool: max 25 connections
- Job CPU: 500m request, 1000m limit
- Job memory: 512Mi request, 1Gi limit
- API timeout: 30 seconds
- Job timeout: 2 hours (7200s)

**Workers:** If you cannot complete your work within these boundaries, return to orchestrator. Never violate boundaries.

---

## Coding Conventions

### Go Project Structure

```
/private/tmp/eval_llm/
├── cmd/
│   └── api/
│       └── main.go           # Entry point
├── internal/
│   ├── handler/              # HTTP handlers (Gin)
│   ├── service/              # Business logic
│   ├── repository/           # Data access (PostgreSQL)
│   ├── model/                # Domain entities
│   ├── config/               # Configuration loading
│   ├── middleware/           # HTTP middleware
│   ├── k8s/                  # Kubernetes client, Job manager
│   ├── cache/                # Redis client
│   └── evaluator/            # OpenCompass integration
│       ├── config.go         # Config generator
│       ├── cli.go            # CLI wrapper
│       ├── parser.go         # Results parser
├── pkg/
│   └── utils/                # Shared utilities
├── configs/
│   ├── config.yaml           # Application config
├── deployments/
│   └── kubernetes/           # K8s manifests (base, overlays)
├── migrations/
│   └── *.sql                 # Database migrations
├── go.mod
├── go.sum
├── Dockerfile
├── Makefile
└── README.md
```

### Code Style

- **Indentation:** Use tabs (Go standard)
- **Naming:** PascalCase for public, camelCase for private
- **Error handling:** Always check errors, never ignore
- **Context:** Pass context.Context as first parameter in service/repository methods
- **JSON:** Use struct tags for JSON serialization (`json:"field_name"`)
- **Comments:** Document public functions, complex logic

### Gin Handler Pattern

```go
func (h *Handler) CreateEvaluation(c *gin.Context) {
    var req CreateEvaluationRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Validate request
    if err := h.validateRequest(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Business logic
    eval, err := h.service.CreateEvaluation(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    
    c.JSON(http.StatusAccepted, eval)
}
```

### Repository Pattern

```go
type EvaluationRepository interface {
    Create(ctx context.Context, eval *Evaluation) error
    GetByID(ctx context.Context, id string) (*Evaluation, error)
    List(ctx context.Context, page, limit int) ([]*Evaluation, int, error)
    UpdateStatus(ctx context.Context, id string, status EvaluationStatus) error
}

type PostgresEvaluationRepository struct {
    db *pgxpool.Pool
}

func (r *PostgresEvaluationRepository) Create(ctx context.Context, eval *Evaluation) error {
    query := `INSERT INTO evaluations (id, model_id, dataset_ids, config, status, created_at)
              VALUES ($1, $2, $3, $4, $5, $6)`
    return r.db.Exec(ctx, query, eval.ID, eval.ModelID, eval.DatasetIDs, eval.Config, eval.Status, eval.CreatedAt).Err()
}
```

---

## Testing Conventions

### Unit Tests

- Use `github.com/stretchr/testify` for assertions
- Mock dependencies (repository, cache) with testify/mock
- Table-driven tests for multiple scenarios
- Test files in same package: `xxx_test.go`

```go
func TestCreateEvaluation(t *testing.T) {
    tests := []struct {
        name    string
        request CreateEvaluationRequest
        wantErr bool
    }{
        {"valid request", CreateEvaluationRequest{Model: "gpt-4", Dataset: "mmlu"}, false},
        {"missing model", CreateEvaluationRequest{Dataset: "mmlu"}, true},
        {"missing dataset", CreateEvaluationRequest{Model: "gpt-4"}, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

- Use `testcontainers-go` for PostgreSQL/Redis containers
- Run integration tests separately: `go test -tags=integration ./...`
- Clean up containers after tests

---

## Testing & Validation Guidance

Instructions for validators from the orchestrator.

### API Testing

**Tool:** curl

**Endpoints to test:**
- POST /api/v1/evaluations (create)
- GET /api/v1/evaluations (list)
- GET /api/v1/evaluations/:id (get)
- GET /api/v1/evaluations/:id/status (status)
- GET /api/v1/evaluations/:id/results (results)
- DELETE /api/v1/evaluations/:id (cancel)
- GET /api/v1/models (models list)
- GET /api/v1/datasets (datasets list)
- GET /health (liveness)
- GET /ready (readiness)

**Required test scenarios:**
1. Valid request creates task (202, UUID format)
2. Missing fields return 400
3. Invalid model/dataset returns 400 with valid list
4. Pagination works correctly
5. Status transitions work (pending → running → completed)
6. Results endpoint returns 409 for pending tasks
7. Cancel works for pending/running, 409 for completed
8. Health returns 200
9. Ready returns 503 when DB disconnected

### Database Testing

**Tool:** psql

**Tests:**
1. Schema verification: `\d evaluations`, `\d models`, `\d results`
2. Foreign key enforcement: INSERT with invalid evaluation_id
3. JSONB operations: query metrics field
4. Index verification: EXPLAIN ANALYZE shows index scan
5. Pagination: LIMIT/OFFSET queries

### Kubernetes Testing

**Tool:** kubectl

**Tests:**
1. Job creation: `kubectl get jobs -n llm-eval`
2. Labels: `kubectl get job -o jsonpath='{.metadata.labels}'`
3. ConfigMap: `kubectl get configmap -o yaml`
4. Secret: `kubectl get secret` (verify base64)
5. Pod logs: `kubectl logs job/<name>` (no API keys)
6. Job status: `kubectl get job -o jsonpath='{.status}'`

### OpenCompass Integration Testing

**Tool:** subprocess (mocked for unit tests)

**Tests:**
1. Config generation: valid Python syntax
2. CLI wrapper: command construction, env vars
3. Result parsing: JSON predictions, CSV summary
4. Error handling: non-zero exit code, timeout

### Cross-Area Flow Testing

**Tools:** curl + kubectl + psql

**Tests:**
1. Full lifecycle: create → Job → complete → results
2. Cancellation: DELETE → Job deleted → status cancelled
3. Error propagation: failure → DB error → API returns error
4. Concurrency: 5 parallel tasks, isolated results
5. Secret security: no keys in logs, API, DB

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `API_PORT` | API server port | 3100 |
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 3105 |
| `DB_NAME` | Database name | evaluations |
| `DB_USER` | Database user | eval_user |
| `DB_PASSWORD` | Database password | (from secret) |
| `REDIS_HOST` | Redis host | localhost |
| `REDIS_PORT` | Redis port | 3106 |
| `K8S_NAMESPACE` | Kubernetes namespace | llm-eval |
| `OPENAI_API_KEY` | OpenAI key | (required for GPT-4) |
| `ANTHROPIC_API_KEY` | Claude key | (optional) |
| `DASHSCOPE_API_KEY` | Qwen key | (optional) |

### Configuration File

```yaml
# configs/config.yaml
server:
  port: 3100
  timeout: 30s

database:
  host: localhost
  port: 3105
  name: evaluations
  max_connections: 25

redis:
  host: localhost
  port: 3106
  ttl: 24h

kubernetes:
  namespace: llm-eval
  job_timeout: 7200s
  job_retries: 3

evaluation:
  container_image: opencompass:latest
  work_dir: /tmp/opencompass_runs
```

---

## API Key Handling

**CRITICAL: Never expose API keys**

1. Keys only in K8s Secrets (base64 encoded)
2. Keys never in DB (only secret reference)
3. Keys never in API responses
4. Keys never in logs (stderr/stdout captured, keys redacted)
5. Keys injected via environment variables in Job pods
6. Config files use env var references: `key='${OPENAI_API_KEY}'`

---

## Status Management

**Status Flow:**
```
pending → running → completed
                ↓
                failed
                ↓
                cancelled
```

**Terminal States:**
- completed (cannot transition)
- failed (cannot transition)
- cancelled (cannot transition)

**Timestamp Updates:**
- created_at: set on creation
- started_at: set when status becomes 'running'
- completed_at: set when status becomes 'completed' or 'failed'
- updated_at: set on every status change

---

## Error Handling

### API Errors

| Status Code | Condition |
|-------------|-----------|
| 400 | Invalid request (missing fields, invalid values) |
| 404 | Task not found |
| 409 | Conflict (already completed/cancelled, results not ready) |
| 500 | Internal error (DB failure, unexpected error) |
| 503 | Service unavailable (DB/Redis down, K8s unreachable) |

### Error Response Format

```json
{
  "error": "Descriptive error message",
  "code": "VALIDATION_ERROR|NOT_FOUND|CONFLICT|INTERNAL_ERROR",
  "request_id": "uuid-for-tracing"
}
```

---

## Dependencies

**Go Dependencies (versions):**
- `github.com/gin-gonic/gin` v1.10+
- `github.com/jackc/pgx/v5` v5.7+
- `github.com/redis/go-redis/v9` v9+
- `k8s.io/client-go` v0.32+
- `github.com/stretchr/testify` v1.10+ (testing)
- `github.com/google/uuid` v1.6+

---

## Common Patterns

### UUID Generation

```go
import "github.com/google/uuid"

taskID := uuid.New().String() // UUID v4
```

### JSONB Handling

```go
// Store as JSONB
metrics := map[string]float64{"accuracy": 0.85, "f1": 0.87}
jsonData, _ := json.Marshal(metrics)
db.Exec(ctx, "INSERT INTO results (evaluation_id, metrics) VALUES ($1, $2)", evalID, jsonData)

// Query JSONB
db.Query(ctx, "SELECT metrics->>'accuracy' FROM results WHERE evaluation_id = $1", evalID)
```

### Pagination

```go
offset := (page - 1) * limit
query := `SELECT * FROM evaluations ORDER BY created_at DESC LIMIT $1 OFFSET $2`
rows, _ := db.Query(ctx, query, limit, offset)

totalQuery := `SELECT COUNT(*) FROM evaluations`
total := db.QueryRow(ctx, totalQuery).Scan()

pages := int(math.Ceil(float64(total) / float64(limit)))
```

---

## Validation Checklist

Before marking a feature complete, verify:

1. **Code compiles:** `go build ./cmd/api`
2. **Tests pass:** `go test ./...`
3. **API works:** curl endpoints manually
4. **DB works:** psql queries succeed
5. **K8s works:** kubectl shows created resources
6. **No boundary violations:** ports, resources within limits
7. **No key exposure:** logs, API, DB don't contain keys
8. **Error handling:** test failure scenarios
9. **Concurrency:** test parallel execution
10. **Documentation:** update README if needed

---

*Worker guidance for LLM Evaluation Backend mission.*
