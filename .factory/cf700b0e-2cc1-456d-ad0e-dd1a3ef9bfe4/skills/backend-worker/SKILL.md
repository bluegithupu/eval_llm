---
name: backend-worker
description: Go backend implementation worker for LLM evaluation system features
---

# Backend Worker

NOTE: Startup and cleanup are handled by `worker-base`. This skill defines the WORK PROCEDURE.

## When to Use This Skill

Use this worker for all backend implementation features:
- Go project initialization and structure setup
- PostgreSQL schema migrations and repository layer
- Redis client configuration and cache helpers
- Gin API handler implementation
- Kubernetes Job/ConfigMap/Secret creation
- OpenCompass config generation and CLI integration
- End-to-end orchestration and testing

## Required Skills, Tools, and Dependencies

**Skills to invoke:**
- None (this worker handles implementation directly)

**Tools/CLIs to execute:**
- `go` (Go toolchain: build, test, mod)
- `psql` (PostgreSQL client for schema verification)
- `redis-cli` (Redis client for cache verification)
- `kubectl` (Kubernetes CLI for Job/resource verification)
- `curl` (API endpoint testing)

**Packages/libraries to use:**
- `github.com/gin-gonic/gin` v1.10+ (web framework)
- `github.com/jackc/pgx/v5` v5.7+ (PostgreSQL driver)
- `github.com/redis/go-redis/v9` v9+ (Redis client)
- `k8s.io/client-go` v0.32+ (Kubernetes API)
- `github.com/google/uuid` v1.6+ (UUID generation)
- `github.com/stretchr/testify` v1.10+ (testing)

**External services:**
- PostgreSQL on localhost:3105
- Redis on localhost:3106
- Kubernetes cluster (via kubectl context)

## Work Procedure

### Step 1: Read Mission Context

Read mission artifacts to understand the feature:
- `{missionDir}/mission.md` - Mission overview
- `{missionDir}/AGENTS.md` - Coding conventions and boundaries
- `{missionDir}/library/architecture.md` - System architecture
- `{missionDir}/validation-contract.md` - Behavioral assertions (find feature's fulfills IDs)
- `{missionDir}/features.json` - Current feature details (preconditions, expectedBehavior, verificationSteps)

### Step 2: Verify Preconditions

Check that all preconditions listed in the feature are satisfied:
- Check previous features in features.json for completion status
- Verify required services are running (psql, redis-cli connectivity)
- Verify required dependencies are installed (go version)

If preconditions not met, return to orchestrator with discoveredIssues.

### Step 3: Write Tests First (TDD - Red Phase)

Write failing tests before implementation:
- Create test file if not exists
- Write test cases covering expectedBehavior
- Run tests: `go test ./...` - expect failures (red)
- Test cases should match assertions in validation-contract.md

**Example for API handler:**
```go
func TestCreateEvaluation_ValidRequest(t *testing.T) {
    // Test valid request returns 202 with task_id
}

func TestCreateEvaluation_MissingModel(t *testing.T) {
    // Test missing model returns 400
}

func TestCreateEvaluation_InvalidModel(t *testing.T) {
    // Test invalid model returns 400 with valid models list
}
```

### Step 4: Implement Feature (TDD - Green Phase)

Implement code to make tests pass:
- Follow project structure in AGENTS.md
- Follow coding conventions (Gin patterns, repository patterns)
- Use required packages (pgx, go-redis, client-go)
- Respect boundaries (ports, resources, API key handling)

**Implementation checklist:**
- [ ] Code follows project structure
- [ ] Uses required packages with correct versions
- [ ] Respects port boundaries (3100, 3105, 3106)
- [ ] Handles errors properly (never ignore)
- [ ] API keys not exposed (in K8s Secrets only)
- [ ] Context passed correctly

### Step 5: Run Tests (TDD - Green Phase Verification)

Run tests to verify implementation:
```bash
go test ./internal/handler -v      # Handler tests
go test ./internal/service -v       # Service tests
go test ./internal/repository -v    # Repository tests
go test ./... -cover                # Full coverage
```

All tests must pass (green). If tests fail, debug and fix.

### Step 6: Manual Verification (Interactive Checks)

Execute verificationSteps from feature definition:
- Use curl for API endpoints
- Use psql for DB queries
- Use redis-cli for cache verification
- Use kubectl for K8s resources

**Example verification commands:**
```bash
# API endpoint
curl -X POST -H 'Content-Type: application/json' -d '{"model":"gpt-4","dataset":"mmlu"}' http://localhost:3100/api/v1/evaluations

# DB query
psql -h localhost -p 3105 -d evaluations -c "\d evaluations"

# Redis query
redis-cli -p 3106 GET eval:status:test-id

# K8s query
kubectl get jobs -n llm-eval -l app=llm-eval
```

Document each verification in handoff.interactiveChecks.

### Step 7: Run Validators

Run code quality checks:
```bash
go build ./cmd/api                    # Build check
go vet ./...                          # Static analysis
go fmt ./...                          # Format check
golangci-lint run ./...               # Lint (if available)
```

All validators must pass. Fix any issues before handoff.

### Step 8: Update Documentation (If Needed)

If feature introduces new concepts or changes behavior:
- Update README.md (if exists)
- Update architecture.md (if structural change)
- Add comments to complex code sections

### Step 9: Prepare Handoff

Document what was implemented:
- salientSummary: 1-4 sentences describing implementation
- whatWasImplemented: Concrete description (50+ chars)
- whatWasLeftUndone: Empty if complete, else describe gaps
- verification.commandsRun: List of commands with exit codes
- verification.interactiveChecks: List of manual checks
- tests.added: Test files and cases
- discoveredIssues: Any issues found (optional)

### Step 10: Commit Changes

If repository code changed:
```bash
git add -A
git commit -m "feat: implement {feature-id}

{description}

Co-authored-by: factory-droid[bot] <138933559+factory-droid[bot]@users.noreply.github.com>"
```

Include commitId in handoff.

## Example Handoff

```json
{
  "salientSummary": "Implemented POST /api/v1/evaluations endpoint with request validation, task creation, and Redis status caching. All tests pass (8 test cases) and manual verification confirms 202 response with UUID task_id.",
  "whatWasImplemented": "Created Gin handler for POST /api/v1/evaluations with validation (model, dataset required fields), repository method for INSERT into evaluations table, Redis client helper for status caching, and unit tests covering valid request, missing fields, and invalid values scenarios.",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {
        "command": "go test ./internal/handler -v",
        "exitCode": 0,
        "observation": "All 8 test cases passed"
      },
      {
        "command": "go build ./cmd/api",
        "exitCode": 0,
        "observation": "Build successful, binary created"
      },
      {
        "command": "go vet ./...",
        "exitCode": 0,
        "observation": "No issues found"
      }
    ],
    "interactiveChecks": [
      {
        "action": "curl -X POST -d '{\"model\":\"gpt-4\",\"dataset\":\"mmlu\"}' http://localhost:3100/api/v1/evaluations",
        "observed": "Response 202 with task_id \"550e8400-e29b-41d4-a716-446655440000\" in UUID format, Location header set"
      },
      {
        "action": "curl -X POST -d '{\"dataset\":\"mmlu\"}' http://localhost:3100/api/v1/evaluations",
        "observed": "Response 400 with error \"model is required\""
      },
      {
        "action": "curl -X POST -d '{\"model\":\"invalid-model\",\"dataset\":\"mmlu\"}' http://localhost:3100/api/v1/evaluations",
        "observed": "Response 400 with error and valid models list: [\"gpt-4\", \"claude\", \"qwen\"]"
      },
      {
        "action": "psql -c \"SELECT id, status FROM evaluations WHERE id='550e8400-e29b-41d4-a716-446655440000'\"",
        "observed": "Record exists with status='pending'"
      },
      {
        "action": "redis-cli -p 3106 GET eval:status:550e8400-e29b-41d4-a716-446655440000",
        "observed": "Returns \"pending\""
      }
    ]
  },
  "tests": {
    "added": [
      {
        "file": "internal/handler/evaluation_handler_test.go",
        "cases": [
          {
            "name": "TestCreateEvaluation_ValidRequest",
            "description": "Valid request returns 202 with UUID task_id"
          },
          {
            "name": "TestCreateEvaluation_MissingModel",
            "description": "Missing model field returns 400 with error"
          },
          {
            "name": "TestCreateEvaluation_MissingDataset",
            "description": "Missing dataset field returns 400 with error"
          },
          {
            "name": "TestCreateEvaluation_InvalidModel",
            "description": "Invalid model returns 400 with valid models list"
          },
          {
            "name": "TestCreateEvaluation_InvalidDataset",
            "description": "Invalid dataset returns 400 with valid datasets list"
          },
          {
            "name": "TestCreateEvaluation_UUIDFormat",
            "description": "Task ID is valid UUID v4 format"
          },
          {
            "name": "TestCreateEvaluation_DatabaseInsert",
            "description": "Evaluation record inserted with status=pending"
          },
          {
            "name": "TestCreateEvaluation_RedisStatus",
            "description": "Redis status key set to pending"
          }
        ]
      }
    ]
  },
  "discoveredIssues": [],
  "commitId": "abc123def456",
  "repoPath": "/private/tmp/eval_llm"
}
```

## When to Return to Orchestrator

Return to orchestrator when:
- **Preconditions not satisfied**: Previous feature incomplete, service unavailable
- **Dependency missing**: Required package can't be installed, tool not available
- **Boundary violation detected**: Port conflict, resource limit exceeded
- **Requirement ambiguous**: Unclear expected behavior, conflicting assertions
- **External blocker**: K8s cluster unreachable, API key not provided
- **Scope exceeded**: Feature requires significantly more work than expected

Do NOT continue implementation if blocked. Return with clear discoveredIssues.

---

*Backend worker skill for LLM evaluation backend mission.*
