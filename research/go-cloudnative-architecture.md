# Go Cloud-Native Architecture: Best Practices for Backend Systems with Kubernetes

**Research Date: May 2026**

This document compiles current best practices for building cloud-native backend systems in Go with Kubernetes, based on 2025-2026 industry recommendations, official documentation, and community insights.

---

## 1. Project Structure

### Standard Go Project Layout

The community-standard project layout (from `golang-standards/project-layout`) provides a proven structure for cloud-native microservices:

```
/myapp
├── cmd/                    # Main applications
│   ├── api/                # API server entry point
│   │   └── main.go
│   └── worker/             # Background worker entry point
│   │   └── main.go
│   └── scheduler/          # Job scheduler entry point
│   │   └── main.go
├── internal/               # Private application code
│   ├── handler/            # HTTP handlers/controllers
│   ├── service/            # Business logic layer
│   ├── repository/         # Data access layer
│   ├── model/              # Domain models/entities
│   ├── config/             # Configuration management
│   └── middleware/          # HTTP middleware
├── pkg/                    # Public libraries (optional, shareable)
│   └── utils/              # Utility functions
│   └── validation/         # Shared validation logic
├── api/                    # API definitions (OpenAPI, protobuf)
│   └── openapi/            # OpenAPI/Swagger specs
│   └── proto/              # Protocol buffer definitions
├── configs/                # Configuration files
│   ├── config.yaml
│   ├── config.dev.yaml
├── deployments/            # Kubernetes manifests
│   ├── kubernetes/
│   │   ├── base/
│   │   ├── overlays/
├── scripts/                # Build, install, deploy scripts
├── test/                   # Additional test data/fixtures
├── go.mod                  # Go module definition
├── go.sum                  # Dependency checksums
├── Dockerfile              # Container build instructions
├── Makefile                # Build automation
└── README.md
```

### Alternative: Flat Structure (for simple services)

For smaller microservices, a flat structure is acceptable:

```
/myapp
├── main.go
├── handler.go
├── service.go
├── repository.go
├── model.go
├── config.go
├── go.mod
├── Dockerfile
└── deployments/
```

### Hexagonal/Clean Architecture Pattern

For enterprise-grade services, consider hexagonal architecture:

```
/myapp
├── cmd/
│   └── api/main.go
├── internal/
│   ├── domain/              # Core business entities & interfaces
│   ├── application/         # Use cases/app services
│   ├── infrastructure/      # External implementations
│   │   ├── postgres/        # DB implementation
│   │   ├── redis/           # Cache implementation
│   │   ├── http/            # HTTP handlers
│   ├── ports/               # Interface definitions
```

**Recommendation:** Use the standard layout with `cmd/` and `internal/` for most cloud-native services. It balances maintainability with Go conventions.

---

## 2. Web Frameworks for REST APIs

### Framework Comparison (2025-2026)

| Framework | Stars | Performance | Pros | Cons | Best For |
|-----------|-------|-------------|------|------|----------|
| **Gin** | 80k+ | Very Fast | Minimal, middleware-rich, widely adopted | Less features than full frameworks | Production APIs, high-throughput services |
| **Echo** | 30k+ | Fast | Minimal, good middleware, extendable | Smaller community than Gin | Microservices, lightweight APIs |
| **Fiber** | 35k+ | Very Fast | Express-like API, built on fasthttp | Compatibility issues with net/http | Node.js developers transitioning to Go |
| **Chi** | 18k+ | Fast | Lightweight, router-focused, stdlib compatible | Minimal features | Simple routing, stdlib preference |
| **Stdlib net/http** | - | Good | Zero dependencies, stable | Manual routing/middleware | Maximum control, no external deps |

### Recommended Framework: Gin (v1.10+)

**Why Gin:**
- Most popular and battle-tested in production
- Excellent performance (benchmark: ~50k req/sec)
- Rich middleware ecosystem
- Good documentation and community support
- JSON binding/validation built-in

```go
// Basic Gin setup with recommended middleware
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/gin-contrib/cors"
    "github.com/gin-contrib/logger"
)

func main() {
    r := gin.New()
    
    // Recommended middleware chain
    r.Use(gin.Recovery())           // Panic recovery
    r.Use(logger.SetLogger())        // Request logging
    r.Use(cors.Default())           // CORS support
    
    // Health endpoints
    r.GET("/health", healthHandler)
    r.GET("/ready", readinessHandler)
    
    // API routes
    v1 := r.Group("/api/v1")
    {
        v1.POST("/tasks", createTask)
        v1.GET("/tasks/:id", getTask)
        v1.GET("/tasks", listTasks)
    }
    
    r.Run(":8080")
}
```

### Alternative: Chi + Stdlib (for maximum simplicity)

```go
// Chi router with stdlib handlers
package main

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
)

func main() {
    r := chi.NewRouter()
    
    r.Use(middleware.Recoverer)
    r.Use(middleware.Logger)
    r.Use(middleware.RequestID)
    
    r.Get("/health", healthHandler)
    r.Route("/api/v1", func(r chi.Router) {
        r.Post("/tasks", createTask)
        r.Get("/tasks/{id}", getTask)
    })
    
    http.ListenAndServe(":8080", r)
}
```

---

## 3. Database Choices for Evaluation Result Storage

### PostgreSQL (Recommended)

**Version: PostgreSQL 17.x (2025)**

**Why PostgreSQL for evaluation results:**
- Strong relational model for structured evaluation data
- Excellent Go driver support (pgx, sqlc)
- JSON/JSONB support for flexible metadata
- Full-text search for result analysis
- Mature tooling and ecosystem
- ACID compliance for data integrity

**Recommended Go Libraries:**

| Library | Version | Use Case |
|---------|---------|----------|
| `github.com/jackc/pgx/v5` | v5.7+ | Primary driver (fast, type-safe) |
| `github.com/sqlc/sqlc` | v1.27+ | SQL-to-Go code generation |
| `github.com/jmoiron/sqlx` | v1.4+ | Extended stdlib patterns |
| `github.com/uptrace/bun` | v1.2+ | ORM alternative |

```go
// Recommended pgx setup with connection pooling
package database

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
    pool *pgxpool.Pool
}

func NewDB(cfg Config) (*DB, error) {
    poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString)
    if err != nil {
        return nil, err
    }
    
    // Production pool settings
    poolConfig.MaxConns = 25
    poolConfig.MinConns = 5
    poolConfig.MaxConnLifetime = 1 * time.Hour
    poolConfig.HealthCheckPeriod = 1 * time.Minute
    
    pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
    return &DB{pool: pool}, err
}
```

### MongoDB (Alternative for document-heavy data)

**Version: MongoDB 8.0 (2025)**

**When to use MongoDB:**
- Highly variable evaluation result schemas
- Need for flexible document structure
- Large amounts of unstructured metadata
- Horizontal scaling requirements

**Recommended Go Driver:**
```go
// Official MongoDB driver setup
import "go.mongodb.org/mongo-driver/mongo"

client, err := mongo.Connect(context.Background(), 
    options.Client().ApplyURI(uri).
    SetMaxPoolSize(100).
    SetMinPoolSize(10))
```

### Comparison Table

| Feature | PostgreSQL | MongoDB |
|---------|------------|---------|
| Schema flexibility | Moderate (JSONB) | High |
| Query performance | Excellent (structured) | Good (document) |
| Go ecosystem | Very mature | Mature |
| Type safety | High (sqlc) | Moderate |
| Transactions | ACID | Multi-document ACID |
| Best for | Structured evaluation data | Flexible/evolving schemas |

**Recommendation:** Use PostgreSQL for evaluation result storage with structured metrics and scores. Use JSONB columns for flexible metadata. Consider MongoDB only if schema is highly variable.

---

## 4. Containerization Best Practices

### Multi-Stage Dockerfile Pattern

```dockerfile
# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy dependency files first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -X main.version=$(git describe --tags)" \
    -a -installsuffix cgo \
    -o /app/server ./cmd/api

# Stage 2: Runtime (Distroless - minimal attack surface)
FROM gcr.io/distroless/static-debian12:latest

# Copy CA certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/server /server

# Non-root user (distroless already sets this)
USER nonroot:nonroot

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/server", "healthcheck"]

ENTRYPOINT ["/server"]
```

### Alternative: Alpine-based (slightly larger but debuggable)

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/server ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/server /server
USER nobody:nobody
HEALTHCHECK CMD wget -q --spider http://localhost:8080/health || exit 1
ENTRYPOINT ["/server"]
```

### Key Optimization Techniques

1. **Use distroless images** - ~2-5MB vs 100MB+ for full Alpine
2. **Multi-stage builds** - Separate build and runtime
3. **Layer caching** - Order COPY commands strategically
4. **Build flags** - `-ldflags="-w -s"` strips debug info (30-50% size reduction)
5. **CGO_ENABLED=0** - Static binary, no libc dependency

### Image Size Comparison

| Base Image | Typical Size | Security |
|------------|--------------|----------|
| Distroless static | 2-5 MB | Best (minimal CVE) |
| Alpine | 5-15 MB | Good |
| Debian slim | 50-100 MB | Moderate |
| Full Debian | 200+ MB | More CVEs |

---

## 5. Kubernetes Integration

### Deployment Structure

```yaml
# deployments/kubernetes/base/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: eval-api
  labels:
    app: eval-api
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: eval-api
  template:
    metadata:
      labels:
        app: eval-api
        version: v1
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
      containers:
      - name: api
        image: eval-api:v1.0.0
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        
        # Resource limits (critical for production)
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        
        # Liveness probe - restart if unhealthy
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
        
        # Readiness probe - don't route traffic until ready
        readinessProbe:
          httpGet:
            path: /ready
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
        
        # Startup probe - allow slow startup
        startupProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 0
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 30  # 150 seconds max startup
        
        # Configuration from ConfigMap
        env:
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: eval-config
              key: log.level
        - name: DB_HOST
          valueFrom:
            configMapKeyRef:
              name: eval-config
              key: database.host
        
        # Secrets for sensitive data
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: eval-secrets
              key: database.password
        
        # Volume mounts
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
      
      volumes:
      - name: config
        configMap:
          name: eval-config
```

### Service Configuration

```yaml
# deployments/kubernetes/base/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: eval-api
spec:
  type: ClusterIP
  selector:
    app: eval-api
  ports:
  - port: 80
    targetPort: http
    name: http
---
apiVersion: v1
kind: Service
metadata:
  name: eval-api-headless
spec:
  type: ClusterIP
  clusterIP: None  # Headless for direct pod access
  selector:
    app: eval-api
  ports:
  - port: 8080
    targetPort: http
```

### ConfigMap and Secrets

```yaml
# deployments/kubernetes/base/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: eval-config
data:
  log.level: "info"
  database.host: "postgres.default.svc.cluster.local"
  database.port: "5432"
  database.name: "evaluations"
  redis.host: "redis.default.svc.cluster.local"
---
apiVersion: v1
kind: Secret
metadata:
  name: eval-secrets
type: Opaque
stringData:
  database.password: "your-password-here"  # Use sealed-secrets or external-secrets in production
  redis.password: "your-redis-password"
  api.key: "your-api-key"
```

### Job vs Deployment Patterns for Evaluation Tasks

| Pattern | Use Case | Example |
|---------|----------|---------|
| **Deployment + Queue** | Continuous processing, auto-scaling | Worker pods read from Redis/NATS queue |
| **Kubernetes Job** | One-off batch tasks, finite work | Single evaluation run, report generation |
| **CronJob** | Scheduled periodic tasks | Daily reports, cleanup jobs |
| **Job with work queue** | Parallel distributed processing | Multi-node evaluation batches |

**Deployment Pattern (recommended for continuous evaluation workers):**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: eval-worker
spec:
  replicas: 5
  template:
    spec:
      containers:
      - name: worker
        image: eval-worker:v1
        env:
        - name: QUEUE_URL
          value: "redis://redis:6379"
```

**Job Pattern (for finite batch tasks):**
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: eval-batch-001
spec:
  completions: 10  # Total work items
  parallelism: 3   # Concurrent workers
  template:
    spec:
      containers:
      - name: worker
        image: eval-worker:v1
        args: ["--batch-id", "001"]
      restartPolicy: OnFailure
```

### Health Check Implementation in Go

```go
package handler

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

// Liveness probe - basic application health
func HealthHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "healthy",
        "timestamp": time.Now().Unix(),
    })
}

// Readiness probe - check dependencies
func ReadinessHandler(db *DB, redis *Redis) gin.HandlerFunc {
    return func(c *gin.Context) {
        checks := map[string]bool{
            "database": checkDatabase(db),
            "redis":    checkRedis(redis),
        }
        
        allHealthy := true
        for _, healthy := range checks {
            if !healthy {
                allHealthy = false
            }
        }
        
        if allHealthy {
            c.JSON(http.StatusOK, gin.H{
                "status": "ready",
                "checks": checks,
            })
        } else {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "status": "not_ready",
                "checks": checks,
            })
        }
    }
}

func checkDatabase(db *DB) bool {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    return db.Ping(ctx) == nil
}

func checkRedis(r *Redis) bool {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    return r.Ping(ctx).Val() == "PONG"
}
```

---

## 6. Queue/Task Systems for Async Execution

### Recommended: Asynq (Redis-based)

**Library: `github.com/hibiken/asynq` v0.25+**

**Pros:**
- Purpose-built for Go
- Redis backend (easy infrastructure)
- Built-in retry, scheduling, priorities
- Web UI for monitoring
- Dead letter queue support

```go
// Task definition
package tasks

import (
    "github.com/hibiken/asynq"
)

const TypeEvaluationRun = "evaluation:run"

type EvaluationPayload struct {
    EvalID    string `json:"eval_id"`
    ModelID   string `json:"model_id"`
    ConfigURL string `json:"config_url"`
}

func NewEvaluationTask(evalID, modelID string) (*asynq.Task, error) {
    payload, err := json.Marshal(EvaluationPayload{
        EvalID:  evalID,
        ModelID: modelID,
    })
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(TypeEvaluationRun, payload), nil
}
```

```go
// Worker/Server setup
package worker

import (
    "github.com/hibiken/asynq"
)

func StartWorker(redisAddr string) {
    srv := asynq.NewServer(
        asynq.RedisClientOpt{Addr: redisAddr},
        asynq.Config{
            Concurrency: 10,
            Queues: map[string]int{
                "critical": 6,
                "default":  3,
                "low":      1,
            },
            RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
                return time.Duration(n) * time.Minute
            },
            IsFailure: func(err error) bool {
                return true // All errors are failures
            },
        },
    )
    
    mux := asynq.NewServeMux()
    mux.HandleFunc(tasks.TypeEvaluationRun, HandleEvaluation)
    
    srv.Run(mux)
}
```

```go
// Client (enqueue tasks)
package client

import "github.com/hibiken/asynq"

func EnqueueEvaluation(client *asynq.Client, evalID, modelID string) error {
    task, err := tasks.NewEvaluationTask(evalID, modelID)
    if err != nil {
        return err
    }
    
    // Enqueue with options
    _, err = client.Enqueue(task,
        asynq.MaxRetry(5),
        asynq.Timeout(30*time.Minute),
        asynq.Queue("critical"),
    )
    return err
}
```

### Alternative: NATS JetStream

**Library: `github.com/nats-io/nats.go` v1.38+**

**Pros:**
- No Redis dependency
- Excellent performance
- Built-in queue groups
- Distributed by design
- Push/pull consumer models

```go
// NATS JetStream setup
import "github.com/nats-io/nats.go"

func SetupNATS(url string) (*nats.Conn, nats.JetStreamContext) {
    nc, _ := nats.Connect(url)
    js, _ := nc.JetStream()
    
    // Create stream for evaluations
    js.AddStream(&nats.StreamConfig{
        Name:     "EVALUATIONS",
        Subjects: []string{"eval.run.*"},
        Retention: nats.WorkQueuePolicy,
    })
    
    return nc, js
}

// Publish task
js.Publish("eval.run.123", payload)

// Consume with queue group (load balanced)
sub, _ := js.QueueSubscribe(
    "eval.run.*", 
    "workers",
    func(m *nats.Msg) {
        // Process evaluation
        m.Ack()
    },
    nats.Durable("eval-worker"),
    nats.ManualAck(),
)
```

### Comparison Table

| System | Pros | Cons | Best For |
|--------|------|------|----------|
| **Asynq (Redis)** | Easy setup, rich features, retries | Redis dependency | Most async task needs |
| **NATS JetStream** | No Redis, distributed, fast | Less task-specific features | Event-driven architectures |
| **Kubernetes Jobs** | No extra infrastructure | Finite work only | One-off batch jobs |
| **Temporal** | Complex workflows, durability | More setup overhead | Long-running workflows |

**Recommendation:** Use Asynq for typical async evaluation tasks. Use Kubernetes Jobs for batch processing. Use NATS for event-driven architectures.

---

## 7. REST API Design Patterns

### Task Management API Structure

```
POST   /api/v1/tasks              # Create new task (async)
GET    /api/v1/tasks              # List tasks
GET    /api/v1/tasks/{id}         # Get task details
DELETE /api/v1/tasks/{id}         # Cancel/delete task
GET    /api/v1/tasks/{id}/status  # Get task status/progress
GET    /api/v1/tasks/{id}/results # Get task results
```

### Async Operation Pattern (Polling)

```go
// Create task - returns immediately with task ID
func CreateTaskHandler(c *gin.Context) {
    var req CreateTaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    taskID := uuid.New().String()
    
    // Enqueue async task
    EnqueueEvaluation(client, taskID, req.ModelID)
    
    // Return task ID immediately (202 Accepted)
    c.JSON(http.StatusAccepted, gin.H{
        "task_id": taskID,
        "status": "pending",
        "status_url": fmt.Sprintf("/api/v1/tasks/%s/status", taskID),
    })
}
```

```go
// Status endpoint - check task progress
func TaskStatusHandler(c *gin.Context) {
    taskID := c.Param("id")
    
    status, err := GetTaskStatus(redis, taskID)
    if err != nil {
        c.JSON(404, gin.H{"error": "task not found"})
        return
    }
    
    response := gin.H{
        "task_id": taskID,
        "status":  status.State,
        "progress": status.Progress,
    }
    
    // Add result URL if complete
    if status.State == "completed" {
        response["result_url"] = fmt.Sprintf("/api/v1/tasks/%s/results", taskID)
    }
    
    c.JSON(http.StatusOK, response)
}
```

### Best Practices

1. **Use 202 Accepted** for async operations that return immediately
2. **Include status_url** for clients to poll progress
3. **Return task_id** as unique identifier
4. **Support cancellation** via DELETE endpoint
5. **Stream results** for large outputs (SSE or chunked)
6. **Rate limiting** on task creation endpoints
7. **Pagination** for list endpoints

### Pagination Pattern

```go
func ListTasksHandler(c *gin.Context) {
    page := c.DefaultQuery("page", "1")
    limit := c.DefaultQuery("limit", "20")
    
    tasks, total, err := db.ListTasks(page, limit)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{
        "data": tasks,
        "meta": gin.H{
            "total":    total,
            "page":     page,
            "limit":    limit,
            "pages":    total / limit,
        },
    })
}
```

---

## 8. Testing Strategies

### Unit Testing with Testify

**Library: `github.com/stretchr/testify` v1.10+**

```go
package service_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
)

// Test suite for organized testing
type EvaluationServiceSuite struct {
    suite.Suite
    service   *EvaluationService
    mockRepo  *MockRepository
    mockQueue *MockQueue
}

func (s *EvaluationServiceSuite) SetupTest() {
    s.mockRepo = new(MockRepository)
    s.mockQueue = new(MockQueue)
    s.service = NewEvaluationService(s.mockRepo, s.mockQueue)
}

func (s *EvaluationServiceSuite) TestCreateEvaluation() {
    s.mockRepo.On("Create", mock.Anything).Return(&Evaluation{ID: "123"}, nil)
    s.mockQueue.On("Enqueue", mock.Anything).Return(nil)
    
    result, err := s.service.CreateEvaluation(context.Background(), &CreateRequest{})
    
    s.Assert().NoError(err)
    s.Assert().Equal("123", result.ID)
    s.mockRepo.AssertExpectations(s.T())
}

func TestEvaluationServiceSuite(t *testing.T) {
    suite.Run(t, new(EvaluationServiceSuite))
}
```

### Mocking Patterns

```go
// Mock repository using testify/mock
type MockRepository struct {
    mock.Mock
}

func (m *MockRepository) GetByID(ctx context.Context, id string) (*Evaluation, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*Evaluation), args.Error(1)
}

func (m *MockRepository) Create(ctx context.Context, eval *Evaluation) error {
    args := m.Called(ctx, eval)
    return args.Error(0)
}
```

### Table-Driven Tests

```go
func TestValidateEvaluation(t *testing.T) {
    tests := []struct {
        name    string
        input   Evaluation
        wantErr bool
    }{
        {
            name: "valid evaluation",
            input: Evaluation{ModelID: "gpt4", Config: "test.yaml"},
            wantErr: false,
        },
        {
            name: "missing model",
            input: Evaluation{Config: "test.yaml"},
            wantErr: true,
        },
        {
            name: "missing config",
            input: Evaluation{ModelID: "gpt4"},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEvaluation(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Testing with Docker

**Library: `github.com/testcontainers/testcontainers-go` v0.35+**

```go
package integration_test

import (
    "context"
    "testing"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func SetupPostgresContainer(t *testing.T) (string, func()) {
    ctx := context.Background()
    
    req := testcontainers.ContainerRequest{
        Image:        "postgres:17-alpine",
        Env:          map[string]string{"POSTGRES_PASSWORD": "test"},
        WaitingFor:   wait.ForLog("database system is ready"),
        ExposedPorts: []string{"5432/tcp"},
    }
    
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    if err != nil {
        t.Fatal(err)
    }
    
    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "5432")
    
    connStr := fmt.Sprintf("postgres://postgres:test@%s:%s/test", host, port.Port())
    
    return connStr, func() {
        container.Terminate(ctx)
    }
}

func TestDatabaseIntegration(t *testing.T) {
    connStr, cleanup := SetupPostgresContainer(t)
    defer cleanup()
    
    db, err := NewDB(connStr)
    assert.NoError(t, err)
    
    // Run integration tests
    err = db.Create(context.Background(), &Evaluation{ID: "test"})
    assert.NoError(t, err)
}
```

### Testing Tool Recommendations

| Tool | Version | Purpose |
|------|---------|---------|
| `stretchr/testify` | v1.10+ | Assertions, mocking, suites |
| `gomock/gomock` | v1.6+ | Interface mocking |
| `testcontainers` | v0.35+ | Integration test containers |
| `go-cmp/cmp` | v0.6+ | Deep equality with diff |
| `stretchr/suite` | v1.10+ | Test organization |
| `gotest.tools` | v3.5+ | Additional assertions |

### Coverage Requirements

```bash
# Run tests with coverage
go test -cover -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Enforce minimum coverage (recommended: 80%)
go test -cover -covermode=atomic ./...
```

---

## 9. Summary Recommendations

### Recommended Stack for Cloud-Native Go Backend

| Component | Recommendation | Version |
|-----------|----------------|---------|
| **Web Framework** | Gin | v1.10+ |
| **Database** | PostgreSQL with pgx | v17 / v5.7+ |
| **Task Queue** | Asynq (Redis) | v0.25+ |
| **Container Base** | Distroless static | latest |
| **Testing** | Testify + testcontainers | v1.10+ / v0.35+ |
| **Config** | Environment + YAML | koanf v1.5+ |
| **Logging** | zerolog or zap | v1.33+ / v1.27+ |
| **Metrics** | Prometheus client | v1.20+ |
| **Tracing** | OpenTelemetry | v1.30+ |

### Key Principles

1. **Keep it simple** - Avoid unnecessary abstraction
2. **Embrace Go conventions** - Use standard patterns
3. **Fail fast** - Validate inputs early
4. **Graceful degradation** - Handle errors at boundaries
5. **Observability first** - Logs, metrics, traces from day one
6. **Test thoroughly** - 80%+ coverage, integration tests
7. **Secure defaults** - Non-root, minimal images, encrypted secrets
8. **Kubernetes-native** - Probes, resources, config via K8s

---

## References

- Go Project Layout: https://github.com/golang-standards/project-layout
- Gin Framework: https://github.com/gin-gonic/gin
- pgx Driver: https://github.com/jackc/pgx
- Asynq Task Queue: https://github.com/hibiken/asynq
- Kubernetes Probes: https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/
- Distroless Images: https://github.com/GoogleContainerTools/distroless
- Testify: https://github.com/stretchr/testify
- Testcontainers: https://github.com/testcontainers/testcontainers-go
- NATS Go Client: https://github.com/nats-io/nats.go

---

*Document generated from current (2025-2026) best practices research.*
