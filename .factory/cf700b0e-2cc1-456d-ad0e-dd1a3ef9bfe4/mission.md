# 云原生LLM评测系统后端开发

# 云原生LLM评测系统后端开发计划

## 概述

构建一个基于Go + Kubernetes的云原生LLM评测系统后端，集成OpenCompass评测框架，支持API模型（OpenAI、Claude、Qwen等）的自动化评测任务管理。

## 核心功能

- **评测任务管理**: 创建、查询、取消评测任务
- **异步执行**: 通过Kubernetes Job执行OpenCompass评测
- **结果存储**: PostgreSQL存储完整评测数据（配置、预测、结果、日志）
- **状态跟踪**: Redis跟踪任务执行状态
- **模型支持**: API模型评测（GPT-4、Claude、Qwen等）

## Milestones

### Milestone 1: 基础架构搭建
- Go项目结构初始化（cmd/api、internal分层）
- PostgreSQL schema设计与迁移（评测表、模型表、数据集表）
- Redis连接与任务状态队列配置
- Gin API框架搭建（健康检查、基础路由）
- Dockerfile与docker-compose配置

### Milestone 2: 评测任务管理API
- POST /api/v1/evaluations - 创建评测任务
- GET /api/v1/evaluations - 列表查询（分页）
- GET /api/v1/evaluations/:id - 获取详情
- GET /api/v1/evaluations/:id/status - 状态查询
- GET /api/v1/evaluations/:id/results - 结果获取
- DELETE /api/v1/evaluations/:id - 取消任务
- GET /api/v1/models - 支持的模型列表
- GET /api/v1/datasets - 支持的数据集列表

### Milestone 3: Kubernetes Job集成
- Job模板设计（OpenCompass容器、资源配置）
- ConfigMap生成（评测配置文件）
- Job创建API调用（client-go）
- Job状态监控与事件捕获
- 完成后结果收集（共享存储或API回调）
- 失败Job的错误处理与重试

### Milestone 4: OpenCompass CLI集成
- OpenCompass Python环境准备（容器镜像）
- CLI子进程调用封装（参数构建、执行）
- 动态配置文件生成（MMEngine格式）
- 结果解析（JSON predictions、CSV summary）
- API模型配置（OpenAI、Claude、Qwen keys管理）
- 输出目录管理与清理

### Milestone 5: 完整端到端流程
- 完整评测流程验证（API模型真实评测）
- 多任务并发测试
- 错误恢复与重试机制
- 性能优化（连接池、并发控制）
- 文档与部署指南

## 环境配置

### 基础设施
- **PostgreSQL**: Docker容器，端口3105，数据库名`evaluations`
- **Redis**: Docker容器，端口3106，用于任务状态跟踪
- **API服务**: 端口3100（开发）
- **Kubernetes**: 本地测试环境（kubectl连接）
- **Go版本**: 1.24+

### Kubernetes资源
- Namespace: `llm-eval`
- Job模板: 每个评测任务独立Pod
- ConfigMap: 评测配置文件
- Secret: API模型密钥
- 无GPU需求（API模型评测）

### 外部服务
- OpenAI API（需要API Key）
- Claude API（需要API Key，可选）
- Qwen API（需要API Key，可选）

## 验证策略

### 单元测试
- Go服务层测试（mock数据库、Redis）
- API handler测试（请求验证、响应格式）
- 配置生成测试

### 集成测试
- PostgreSQL数据存储验证
- Redis状态更新验证
- Kubernetes Job创建验证

### 端到端测试
- 完整评测流程：API创建 → K8s Job → OpenCompass执行 → 结果存储
- API模型真实评测（使用测试API Key）
- 结果数据完整性验证

### 测试工具
- `go test` + testify（单元测试）
- testcontainers（PostgreSQL/Redis集成测试）
- kubectl（K8s Job验证）
- curl（API端点测试）

## 端口与边界

### 端口范围
- API服务: 3100
- PostgreSQL: 3105
- Redis: 3106
- 避免端口冲突（检查现有8080、5000等）

### 资源边界
- PostgreSQL表结构: evaluations、models、datasets、results、predictions
- Redis队列: eval:status:{id}, eval:queue
- K8s资源: 在`llm-eval` namespace中

## 非功能性需求

### 性能
- API响应时间 < 100ms（查询类）
- 支持10个并发评测任务
- PostgreSQL连接池（25连接）

### 可观测性
- 健康检查端点（/health、/ready）
- 任务执行日志（存储到数据库）
- Prometheus metrics准备（可选）

### 安全
- 无认证（内部服务）
- API密钥通过K8s Secret管理
- 数据库密码加密存储

### 容错
- Job失败重试（最多3次）
- API超时处理（30s）
- 数据库连接重连