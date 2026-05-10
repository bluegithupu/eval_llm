#!/bin/bash

# Environment setup script for LLM evaluation backend mission
# This script is idempotent - can be run multiple times safely

set -e

echo "=== Initializing LLM Evaluation Backend Environment ==="

# Check Go version
echo "Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo "ERROR: Go not installed. Please install Go 1.24+"
    exit 1
fi
GO_VERSION=$(go version | grep -oP 'go1\.\d+' | head -1)
echo "Go version: $GO_VERSION"

# Check Docker
echo "Checking Docker..."
if ! command -v docker &> /dev/null; then
    echo "WARNING: Docker not available. Services will need manual setup."
else
    echo "Docker is available"
fi

# Check kubectl
echo "Checking kubectl..."
if ! command -v kubectl &> /dev/null; then
    echo "WARNING: kubectl not installed. Kubernetes integration will need manual setup."
else
    echo "kubectl version: $(kubectl version --client --short 2>/dev/null || kubectl version --client | head -1)"
fi

# Check psql
echo "Checking PostgreSQL client..."
if ! command -v psql &> /dev/null; then
    echo "WARNING: psql not installed. Database verification will need alternative method."
else
    echo "psql is available"
fi

# Check redis-cli
echo "Checking Redis client..."
if ! command -v redis-cli &> /dev/null; then
    echo "WARNING: redis-cli not installed. Cache verification will need alternative method."
else
    echo "redis-cli is available"
fi

# Create project directories (idempotent)
echo "Creating project directories..."
mkdir -p cmd/api
mkdir -p internal/handler
mkdir -p internal/service
mkdir -p internal/repository
mkdir -p internal/model
mkdir -p internal/config
mkdir -p internal/middleware
mkdir -p internal/k8s
mkdir -p internal/cache
mkdir -p internal/evaluator
mkdir -p pkg/utils
mkdir -p configs
mkdir -p deployments/kubernetes/base
mkdir -p deployments/kubernetes/overlays
mkdir -p migrations
mkdir -p test
mkdir -p bin

# Initialize go.mod if not exists
if [ ! -f "go.mod" ]; then
    echo "Initializing go.mod..."
    go mod init github.com/eval_llm/backend
fi

# Install dependencies (idempotent)
echo "Installing Go dependencies..."
go get github.com/gin-gonic/gin@v1.10
go get github.com/jackc/pgx/v5@latest
go get github.com/redis/go-redis/v9@latest
go get k8s.io/client-go@v0.32
go get github.com/google/uuid@latest
go get github.com/stretchr/testify@v1.10
go mod tidy

# Create Kubernetes namespace if not exists
echo "Creating Kubernetes namespace..."
if command -v kubectl &> /dev/null; then
    kubectl create namespace llm-eval --dry-run=client -o yaml | kubectl apply -f - || true
    echo "Namespace llm-eval ready"
fi

# Create default config file if not exists
if [ ! -f "configs/config.yaml" ]; then
    echo "Creating default config..."
    cat > configs/config.yaml << 'EOF'
server:
  port: 3100
  timeout: 30s

database:
  host: localhost
  port: 3105
  name: evaluations
  user: eval_user
  password: eval_pass
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
EOF
fi

# Create placeholder main.go if not exists
if [ ! -f "cmd/api/main.go" ]; then
    echo "Creating placeholder main.go..."
    cat > cmd/api/main.go << 'EOF'
package main

import (
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()
    
    // Health endpoints (placeholder)
    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "healthy"})
    })
    
    r.GET("/ready", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ready"})
    })
    
    // API routes (to be implemented)
    r.Run(":3100")
}
EOF
fi

echo "=== Environment initialization complete ==="
echo ""
echo "Next steps:"
echo "1. Start PostgreSQL: docker run -d --name eval-postgres -e POSTGRES_USER=eval_user -e POSTGRES_PASSWORD=eval_pass -e POSTGRES_DB=evaluations -p 3105:5432 postgres:17-alpine"
echo "2. Start Redis: docker run -d --name eval-redis -p 3106:6379 redis:7-alpine"
echo "3. Run migrations: psql -h localhost -p 3105 -U eval_user -d evaluations -f migrations/*.sql"
echo "4. Build and run: go build ./cmd/api && PORT=3100 ./bin/api"
