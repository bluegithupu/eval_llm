# System Architecture: LLM Evaluation Backend

This document describes the high-level architecture of the cloud-native LLM evaluation backend system.

---

## Overview

The LLM Evaluation Backend is a cloud-native system built with Go + Kubernetes + PostgreSQL + Redis, integrated with the OpenCompass LLM evaluation framework. It provides a REST API for managing evaluation tasks, executes evaluations as Kubernetes Jobs, and persists results for analysis.

---

## Components

### 1. API Server (Go + Gin)

The API server provides the REST interface for task management.

**Responsibilities:**
- Handle HTTP requests for evaluation CRUD operations
- Validate request payloads (model, dataset, parameters)
- Create Kubernetes Jobs for evaluation execution
- Query PostgreSQL for task history and results
- Update Redis for task status caching
- Return paginated, structured JSON responses

**Technology:**
- Go 1.24+
- Gin web framework
- client-go for Kubernetes API
- pgx for PostgreSQL
- go-redis for Redis

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

---

### 2. PostgreSQL Database

PostgreSQL stores persistent evaluation data and configuration.

**Tables:**
| Table | Purpose |
|-------|---------|
| `evaluations` | Task records (id, model_id, datasets, status, timestamps) |
| `models` | Model definitions (name, type, provider, api_key_ref) |
| `datasets` | Dataset definitions (name, description, config_template) |
| `results` | Evaluation results (accuracy, metrics JSONB, summary) |
| `predictions` | Per-sample predictions (question, prediction, answer, correct) |
| `logs` | Task execution logs (timestamp, level, message) |

**Key Fields:**
- `evaluations.status`: Enum states (pending, running, completed, failed, cancelled)
- `results.metrics`: JSONB for flexible metric storage
- `evaluations.config`: JSONB for request parameters

**Cross-Feature Dependencies:**
- `evaluation_repository.UpdateStatus()` relies on database triggers (from database-schema feature) to automatically set `started_at` when status becomes 'running' and `completed_at` when status becomes terminal state
- Foreign key columns have indexes for JOIN performance (idx_evaluations_model_id, idx_predictions_evaluation_id, idx_predictions_dataset_id, idx_logs_evaluation_id)

---

### 3. Redis Cache

Redis provides fast status tracking and task queue management.

**Key Patterns:**
| Pattern | Purpose |
|---------|---------|
| `eval:status:{id}` | Current task status (pending/running/completed/failed) |
| `eval:progress:{id}` | Progress percentage (0-100) |
| `eval:queue` | Pending task queue (optional, for prioritization) |

**TTL:**
- Status keys expire after 24 hours (DB is authoritative)
- Queue entries removed when task starts

---

### 4. Kubernetes Job Executor

Kubernetes Jobs execute OpenCompass evaluations in isolated pods.

**Job Flow:**
1. API creates Job via client-go
2. Job spawns Pod with OpenCompass container
3. Pod runs evaluation (CLI subprocess)
4. Pod writes results to shared volume
5. API collects results, stores in DB
6. Job cleanup (TTL or manual)

**Resources Created:**
| Resource | Purpose |
|----------|---------|
| Job | Orchestrates Pod execution |
| ConfigMap | OpenCompass configuration file (MMEngine format) |
| Secret | API model keys (OpenAI, Claude, Qwen) |
| Pod | Runs evaluation container |

**Labels:**
- `app=llm-eval`
- `eval-id={task_id}`
- `model={model_name}`

---

### 5. OpenCompass CLI Integration

OpenCompass is invoked via CLI subprocess inside Kubernetes Pods.

**Integration Approach:**
- Generate MMEngine config file (Python format)
- Execute `python run.py --datasets ... --work-dir ...`
- Inject API keys via environment variables
- Capture stdout/stderr for logs
- Parse results from output directory (JSON predictions, CSV summary)

**Configuration Generation:**
```python
# Generated config.py structure
from opencompass.models import OpenAI

models = [
    dict(
        type=OpenAI,
        path='gpt-4',
        max_seq_len=2048,
        max_out_len=100,
        key='${OPENAI_API_KEY}',  # env var
        run_cfg=dict(num_gpus=0),
    )
]

datasets = [...]  # Dataset configs (MMLU, HellaSwag)
```

---

## Data Flows

### Evaluation Creation Flow

```
User → POST /api/v1/evaluations
      ↓
API validates request
      ↓
API INSERT into evaluations (status=pending)
      ↓
API SET Redis eval:status:{id}=pending
      ↓
API returns 202 Accepted with task_id
```

### Job Execution Flow

```
API → Create K8s Job
      ↓
K8s → Create ConfigMap (config.py)
      ↓
K8s → Create Secret (API keys)
      ↓
K8s → Spawn Pod
      ↓
Pod → Execute OpenCompass CLI
      ↓
Pod → Write results to /results/
      ↓
API → Poll Job status (via K8s API)
      ↓
API → Collect results when complete
      ↓
API → INSERT into results, predictions
      ↓
API → UPDATE evaluations status=completed
      ↓
API → SET Redis eval:status:{id}=completed
```

### Status Query Flow

```
User → GET /api/v1/evaluations/:id/status
      ↓
API → GET Redis eval:status:{id}
      ↓
If missing → Query K8s Job status → Update Redis
      ↓
API → Return status, progress
```

---

## Key Invariants

1. **Status Consistency**: Redis status mirrors DB status (eventually consistent)
2. **Task Uniqueness**: Each task has unique UUID, no duplicate task_id
3. **Foreign Key Integrity**: results/predictions must reference valid evaluation_id
4. **Job Ownership**: Each Job has owner references to ConfigMap/Secret for cleanup
5. **Key Security**: API keys never in DB, only in K8s Secrets, never in logs
6. **Terminal States**: completed/failed/cancelled are terminal (no further transitions)

---

## Resource Limits

| Resource | Limit |
|----------|-------|
| PostgreSQL connections | 25 (pool) |
| Redis memory | 100MB |
| Job CPU | 500m request, 1000m limit |
| Job memory | 512Mi request, 1Gi limit |
| API timeout | 30s |
| Job timeout | 2h (activeDeadlineSeconds) |
| Job retries | 3 (backoffLimit) |

---

## Failure Handling

| Failure | Response |
|---------|----------|
| OpenCompass CLI error | Mark Job failed, store stderr in DB |
| K8s Job creation error | Return 503, mark task failed |
| PostgreSQL unavailable | Return 503, Job continues (buffer locally) |
| Redis unavailable | Fall back to K8s Job status polling |
| API key invalid | Mark evaluation failed with auth error |
| Job timeout | Mark failed with timeout error |

---

## Security

- API is internal (no authentication required)
- API keys stored in K8s Secrets (base64 encoded)
- Jobs run as non-root user
- Secrets mounted as volumes, not env vars (optional)
- No keys in API responses or DB records

---

## Scalability

- API handles 100+ concurrent requests
- 10+ concurrent Jobs supported
- Pagination for large result sets
- Efficient DB queries with indexes (status, created_at)
- Redis caching reduces DB load

---

## Observability

- `/health` endpoint (liveness)
- `/ready` endpoint (DB + Redis connectivity)
- Structured JSON logging
- Prometheus metrics (optional)
- Task execution logs stored in DB

---

## Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go | 1.24+ | Backend runtime |
| Gin | v1.10+ | Web framework |
| pgx | v5.7+ | PostgreSQL driver |
| go-redis | v9+ | Redis client |
| client-go | v0.32+ | Kubernetes API |
| PostgreSQL | 17 | Primary database |
| Redis | 7 | Status cache |
| OpenCompass | latest | LLM evaluation framework |
| Python | 3.10+ | OpenCompass runtime |

---

*Architecture document for LLM Evaluation Backend mission.*
