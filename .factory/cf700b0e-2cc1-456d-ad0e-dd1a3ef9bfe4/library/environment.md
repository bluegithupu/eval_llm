# Environment

Environment variables, external dependencies, and setup notes for LLM evaluation backend.

**What belongs here:** Required env vars, external API keys/services, dependency quirks, platform-specific notes.
**What does NOT belong here:** Service ports/commands (use `services.yaml`).

---

## Required Environment Variables

| Variable | Required | Description | Source |
|----------|----------|-------------|--------|
| `API_PORT` | No | API server port (default: 3100) | Application config |
| `DB_HOST` | No | PostgreSQL host (default: localhost) | Application config |
| `DB_PORT` | No | PostgreSQL port (default: 3105) | Application config |
| `DB_NAME` | No | Database name (default: evaluations) | Application config |
| `DB_USER` | No | Database user (default: eval_user) | Application config |
| `DB_PASSWORD` | Yes | Database password | Docker secret / config |
| `REDIS_HOST` | No | Redis host (default: localhost) | Application config |
| `REDIS_PORT` | No | Redis port (default: 3106) | Application config |
| `K8S_NAMESPACE` | No | Kubernetes namespace (default: llm-eval) | Application config |
| `OPENAI_API_KEY` | Yes* | OpenAI API key for GPT-4 evaluation | User-provided secret |
| `ANTHROPIC_API_KEY` | No* | Anthropic API key for Claude evaluation | User-provided secret |
| `DASHSCOPE_API_KEY` | No* | DashScope API key for Qwen evaluation | User-provided secret |

*Required only when evaluating respective API models.

---

## External Dependencies

### PostgreSQL 17

- **Purpose:** Primary data store for evaluations, results, predictions
- **Connection:** localhost:3105
- **User:** eval_user
- **Database:** evaluations
- **Pool:** Max 25 connections
- **Setup:** Docker container (see services.yaml)

### Redis 7

- **Purpose:** Status caching, task queue (optional)
- **Connection:** localhost:3106
- **TTL:** 24 hours for status keys
- **Setup:** Docker container (see services.yaml)

### Kubernetes Cluster

- **Purpose:** Job execution for OpenCompass evaluations
- **Namespace:** llm-eval
- **Context:** Use existing kubectl context
- **Requirements:** client-go v0.32+ for API access

### OpenCompass

- **Purpose:** LLM evaluation framework
- **Version:** Latest (pip install opencompass)
- **Container Image:** opencsghq/opencompass or custom
- **Python Version:** 3.10+
- **Integration:** CLI subprocess execution

---

## Platform Notes

### macOS (Current Environment)

- Go installed at `/opt/homebrew/bin/go` (version 1.24.4)
- kubectl installed at `/opt/homebrew/bin/kubectl` (version 1.32.3)
- Docker may not be running (check with `docker ps`)
- Port conflicts possible (check with `lsof -i -P -n | grep LISTEN`)
- **psql and pg_isready CLI tools are not installed locally** - Use `docker exec eval-postgres psql` for database operations inside the PostgreSQL container

### Port Allocation

- API: 3100 (avoid 8080, 5000, 7000 conflicts)
- PostgreSQL: 3105 (avoid default 5432)
- Redis: 3106 (avoid default 6379)

---

## API Key Management

### Key Sources

1. **OpenAI API Key:**
   - Get from https://platform.openai.com/api-keys
   - Store in Kubernetes Secret or env var
   - Never store in DB or config files

2. **Anthropic API Key (Claude):**
   - Get from https://console.anthropic.com
   - Optional, only needed for Claude evaluation

3. **DashScope API Key (Qwen):**
   - Get from https://dashscope.console.aliyun.com
   - Optional, only needed for Qwen evaluation

### Key Security

- Keys injected via Kubernetes Secrets (base64 encoded)
- Keys mounted as environment variables in Job pods
- Keys never in API responses, DB records, or logs
- Config files reference env vars: `key='${OPENAI_API_KEY}'`

---

## Setup Checklist

1. Go 1.24+ installed
2. kubectl configured with cluster access
3. Docker running (for PostgreSQL/Redis)
4. PostgreSQL container started (port 3105)
5. Redis container started (port 3106)
6. Kubernetes namespace `llm-eval` created
7. API keys obtained (for models to evaluate)
8. OpenCompass container image available

---

*Environment documentation for LLM evaluation backend.*
