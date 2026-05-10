# LLM Evaluation Backend

云原生 LLM 评测系统后端，基于 Go + Kubernetes 构建，集成 OpenCompass 评测框架，支持 API 模型（OpenAI、Claude、Qwen 等）的自动化评测任务管理。

## 功能特性

- **评测任务管理**: 创建、查询、取消评测任务
- **异步执行**: 通过 Kubernetes Job 执行 OpenCompass 评测
- **结果存储**: PostgreSQL 存储完整评测数据（配置、预测、结果）
- **状态跟踪**: Redis 缓存任务执行状态
- **模型支持**: API 模型评测（GPT-4、Claude、Qwen 等）
- **并发处理**: 支持多任务并发评测，乐观锁机制防止竞态条件
- **安全设计**: API 密钥通过 K8s Secret 管理，不在日志、API、数据库中暴露

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.25 |
| Web框架 | Gin |
| 数据库 | PostgreSQL (pgx/v5) |
| 缓存 | Redis (go-redis/v9) |
| 容器编排 | Kubernetes (client-go v0.32) |
| 测试 | stretchr/testify |
| 评测框架 | OpenCompass |

## 项目结构

```
eval_llm/
├── cmd/api/main.go           # 应用入口
├── internal/
│   ├── handler/              # HTTP handlers (Gin)
│   ├── service/              # 业务逻辑层
│   ├── repository/           # 数据访问层
│   ├── model/                # 领域模型
│   ├── config/               # 配置加载
│   ├── cache/                # Redis 客户端
│   ├── k8s/                  # Kubernetes 集成
│   │   ├── client.go         # K8s 客户端
│   │   ├── configmap/        # ConfigMap 生成器
│   │   ├── secret/           # Secret 管理器
│   │   ├── job/              # Job 创建器
│   │   └── monitor/          # Job 状态监控
│   └── evaluator/            # OpenCompass 集成
│       ├── config.go         # 配置生成器
│       ├── cli.go            # CLI 包装器
│       └── parser.go         # 结果解析器
├── pkg/utils/                # 共享工具
├── configs/config.yaml       # 应用配置
├── migrations/               # 数据库迁移
├── deployments/              # K8s 部署清单
├── go.mod
├── go.sum
└── README.md
```

## 快速开始

### 前置要求

- Go 1.25+
- Docker
- Kubernetes 集群 (kind/minikube)
- PostgreSQL 17
- Redis 7

### 安装依赖

```bash
go mod download
```

### 启动基础设施

```bash
# PostgreSQL
docker run -d --name eval-postgres \
  -e POSTGRES_USER=eval_user \
  -e POSTGRES_PASSWORD=eval_pass \
  -e POSTGRES_DB=evaluations \
  -p 3105:5432 postgres:17-alpine

# Redis
docker run -d --name eval-redis \
  -p 3106:6379 redis:7-alpine

# Kubernetes (kind)
kind create cluster --name llm-eval
kubectl create namespace llm-eval
```

### 运行数据库迁移

```bash
docker exec eval-postgres psql -U eval_user -d evaluations \
  -f /path/to/migrations/001_init_schema.sql
```

### 构建和运行

```bash
# 构建
go build -o bin/api ./cmd/api

# 运行
./bin/api
# 或
go run ./cmd/api
```

服务将在 `http://localhost:3100` 启动。

## API 文档

### 健康检查

| 端点 | 方法 | 描述 |
|------|------|------|
| `/health` | GET | 服务存活检查 |
| `/ready` | GET | 服务就绪检查（检查 DB + Redis） |

### 评测任务

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/evaluations` | POST | 创建评测任务 |
| `/api/v1/evaluations` | GET | 列表查询（分页） |
| `/api/v1/evaluations/:id` | GET | 获取详情 |
| `/api/v1/evaluations/:id/status` | GET | 状态查询 |
| `/api/v1/evaluations/:id/results` | GET | 结果获取 |
| `/api/v1/evaluations/:id` | DELETE | 取消任务 |

### 模型和数据集

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/models` | GET | 支持的模型列表 |
| `/api/v1/datasets` | GET | 支持的数据集列表 |

### 创建评测任务示例

```bash
curl -X POST http://localhost:3100/api/v1/evaluations \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "dataset": "mmlu"}'

# 响应
# HTTP 202 Accepted
# {
#   "task_id": "550e8400-e29b-41d4-a716-446655440000",
#   "status": "pending"
# }
```

## 支持的模型

| 模型 | Provider | 类型 |
|------|----------|------|
| gpt-4 | openai | api |
| gpt-4-turbo | openai | api |
| gpt-3.5-turbo | openai | api |
| claude-3-opus | anthropic | api |
| claude-3-sonnet | anthropic | api |
| claude-3-haiku | anthropic | api |
| qwen-max | dashscope | api |
| qwen-plus | dashscope | api |
| qwen-turbo | dashscope | api |

## 支持的数据集

| 数据集 | 描述 |
|--------|------|
| mmlu | Massive Multitask Language Understanding |
| hellaswag | Commonsense NLI |
| humaneval | Code Generation Benchmark |
| gsm8k | Grade School Math |
| ceval | Chinese Evaluation |

## 配置

### 环境变量

| 变量 | 描述 | 默认值 |
|------|------|--------|
| `API_PORT` | API 服务端口 | 3100 |
| `DB_HOST` | PostgreSQL 主机 | localhost |
| `DB_PORT` | PostgreSQL 端口 | 3105 |
| `DB_NAME` | 数据库名 | evaluations |
| `DB_USER` | 数据库用户 | eval_user |
| `DB_PASSWORD` | 数据库密码 | - |
| `REDIS_HOST` | Redis 主机 | localhost |
| `REDIS_PORT` | Redis 端口 | 3106 |
| `K8S_NAMESPACE` | K8s namespace | llm-eval |

### 配置文件

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

## 开发指南

### 运行测试

```bash
# 所有测试
go test ./...

# 带覆盖率
go test -cover ./...

# 特定包
go test ./internal/handler -v

# 高负载测试
go test ./internal/service -run TestHighLoad -v
```

### 代码风格

```bash
# 格式化
go fmt ./...

# 静态检查
go vet ./...

# 构建
go build ./cmd/api
```

### 测试覆盖率

| 包 | 覆盖率 |
|-----|--------|
| cache | 91.4% |
| evaluator | 91.7% |
| handler | 85.9% |
| k8s | 98.3%+ |
| repository | 79.4% |

## 架构设计

### 状态流转

```
pending → running → completed
              ↓
              failed
              ↓
              cancelled
```

### 组件交互

```
1. API Layer (handler/) 接收 HTTP 请求
2. Orchestrator (service/) 创建 K8s Job
3. K8s Integration (k8s/) 管理 Job 生命周期
   - ConfigMap: 评测配置
   - Secret: API 密钥
   - Job: 执行容器
   - Monitor: 状态监控
4. OpenCompass (evaluator/) 执行评测
5. Repository (repository/) 存储结果到 PostgreSQL
6. Cache (cache/) 缓存状态到 Redis
```

## Kubernetes 集成

### Job 规格

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: eval-job-{evaluation-id}
  namespace: llm-eval
  labels:
    app: llm-eval
    eval-id: {id}
    model: gpt-4
    dataset: mmlu
spec:
  backoffLimit: 3
  activeDeadlineSeconds: 7200
  template:
    spec:
      restartPolicy: OnFailure
      containers:
      - name: opencompass
        image: opencompass:latest
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        volumes:
        - name: config
          configMap: {eval-config-{id}}
        - name: api-keys
          secret: {eval-secret-{id}}
```

## 安全设计

### API 密钥管理

1. 密钥存储在 K8s Secrets（base64 编码）
2. 数据库只存储密钥引用，不存储实际密钥
3. API 响应不包含密钥
4. 日志不包含密钥
5. Job Pod 通过文件挂载读取密钥

### 错误处理

| 状态码 | 条件 |
|--------|------|
| 400 | 请求无效（缺少字段、无效值） |
| 404 | 任务不存在 |
| 409 | 冲突（已完成/取消、结果未就绪） |
| 500 | 内部错误 |
| 503 | 服务不可用（DB/Redis 断连） |

## 性能指标

- API 响应时间 < 100ms（查询类）
- 支持 10+ 并发评测任务
- PostgreSQL 连接池：25 连接
- 结果分页支持 10,000+ 预测记录

## 故障排查

### 常见问题

**Job 未创建**
```bash
# 检查 K8s 连接
kubectl cluster-info
kubectl get ns llm-eval

# 检查日志
kubectl logs -n llm-eval job/eval-job-{id}
```

**状态卡在 pending**
```bash
# 检查 Job 状态
kubectl get jobs -n llm-eval

# 检查 Pod 日志
kubectl logs -n llm-eval -l app=llm-eval
```

**Redis 连接失败**
```bash
# 检查 Redis
redis-cli -p 3106 ping
```

## 许可证

MIT License

## 贡献指南

参考 [AGENTS.md](AGENTS.md) 获取开发约定和验证清单。

## 构建可视化

查看 [build-process-visualization.html](build-process-visualization.html) 了解完整的构建过程。

---

*云原生 LLM 评测系统后端 | 基于 Go + Kubernetes + OpenCompass*
