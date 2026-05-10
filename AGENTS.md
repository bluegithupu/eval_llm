# Repository Guidelines

## Project Structure & Module Organization

A Go backend service for LLM evaluation using OpenCompass framework.

- **cmd/api** - Application entry point (`main.go`)
- **internal/** - Core application code:
  - `handler/` - HTTP request handlers (Gin framework)
  - `service/` - Business logic layer (orchestrator for evaluation lifecycle)
  - `repository/` - Database access layer (PostgreSQL with pgx)
  - `k8s/` - Kubernetes client, Job management, Secret/ConfigMap generation, monitoring
  - `evaluator/` - OpenCompass CLI wrapper and results parser
  - `model/` - Domain models (Evaluation, Model, Dataset, Result)
  - `cache/` - Redis client for status caching
  - `config/` - Configuration loading
- **pkg/utils/** - Shared utilities
- **migrations/** - Database migration scripts
- **configs/** - Configuration files (`config.yaml`)
- **deployments/** - Deployment manifests

## Build, Test, and Development Commands

```bash
# Build the API server binary
go build -o bin/api ./cmd/api

# Run the API server
go run ./cmd/api

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run tests for a specific package
go test -v ./internal/handler

# Format code
go fmt ./...

# Check for issues
go vet ./...
```

## Coding Style & Naming Conventions

- **Language**: Go 1.25
- **Formatting**: Standard Go formatting (`go fmt`)
- **Framework**: Gin for HTTP routing
- **Database**: PostgreSQL via pgx/v5 driver
- **Testing**: stretchr/testify for assertions and mocks
- **Naming**: PascalCase for public functions/types, camelCase for private
- **Error handling**: Return errors explicitly, use structured logging (`slog`)
- **JSON tags**: Use snake_case for JSON field names in structs

## Testing Guidelines

- Test files placed alongside source files (`*_test.go`)
- Use testify/assert for assertions and testify/mock for mocks
- Test naming pattern: `Test<FunctionName>_<Scenario>` (e.g., `TestCreateEvaluation_ValidRequest`)
- Tests reference spec IDs like `VAL-API-001` for traceability
- Mock structs follow pattern: `Mock<Type>` (e.g., `MockEvaluationRepository`)
- Use `gin.SetMode(gin.TestMode)` for HTTP handler tests

## Commit & Pull Request Guidelines

- **Format**: `<type>: <description>`
- **Types**: `feat`, `fix`, `style`, `chore`, `docs`, `test`, `refactor`
- **Examples** from history:
  - `feat: implement evaluation orchestrator for end-to-end evaluation lifecycle`
  - `fix: K8s Job creation not working`
  - `style: fix formatting in concurrency_test.go`
  - `feat: add large offset pagination test with 10,000+ predictions`

## Architecture Overview

The system orchestrates LLM evaluations via Kubernetes Jobs:

1. **API Layer** (`handler/`) receives requests to create/list/cancel evaluations
2. **Orchestrator** (`service/orchestrator.go`) manages evaluation lifecycle:
   - Creates K8s Jobs with OpenCompass container
   - Monitors Job status via event store
   - Collects results when Jobs complete
3. **K8s Integration** (`k8s/`) handles:
   - Secret management for API keys
   - ConfigMap generation for OpenCompass config
   - Job creation and monitoring
   - Orphaned Job cleanup
4. **Repository Layer** persists evaluations and results to PostgreSQL
5. **Cache Layer** stores status in Redis for quick polling
