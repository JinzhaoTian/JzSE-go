# 技术选型

## 1. 核心技术栈

### 1.1 编程语言

| 技术 | 选型 | 理由 |
|------|------|------|
| 主语言 | **Go 1.21+** | 高性能、原生并发支持、云原生生态完善、部署简单 |

### 1.2 Web 框架

| 组件 | 选型 | 理由 |
|------|------|------|
| HTTP 框架 | **Gin** | 高性能、中间件生态丰富、项目已使用 |
| API 规范 | **RESTful + gRPC** | REST 对外暴露，gRPC 服务内部通信 |

### 1.3 数据存储

| 组件 | 选型 | 备选 | 使用场景 |
|------|------|------|----------|
| 元数据存储 | **etcd** | Consul | 分布式KV存储，全局元数据、配置管理 |
| 本地元数据缓存 | **BadgerDB** | BoltDB | 嵌入式KV数据库，本地元数据持久化 |
| 文件存储 | **本地文件系统** | MinIO | 区域内文件实际存储 |
| 关系型数据 | **PostgreSQL** | - | 用户、权限等结构化数据 |

### 1.4 消息队列与事件

| 组件 | 选型 | 备选 | 使用场景 |
|------|------|------|----------|
| 事件总线 | **NATS** | Kafka | 区域间事件通知、轻量级消息传递 |
| 任务队列 | **Asynq** | Machinery | 异步任务处理、同步任务调度 |

### 1.5 服务发现与配置

| 组件 | 选型 | 理由 |
|------|------|------|
| 服务发现 | **etcd** | 与元数据存储统一 |
| 配置管理 | **Viper** | Go 生态标准，支持多种格式 |

## 2. 基础设施

### 2.1 容器化与编排

| 组件 | 选型 | 理由 |
|------|------|------|
| 容器运行时 | **Docker** | 标准容器化方案 |
| 容器编排 | **Kubernetes** | 云原生标准，方便多区域部署 |
| Helm Charts | 自定义 | 简化部署配置 |

### 2.2 可观测性

| 组件 | 选型 | 使用场景 |
|------|------|----------|
| 日志 | **Zap** + **Loki** | 结构化日志、集中式日志收集 |
| 指标 | **Prometheus** + **Grafana** | 系统监控、告警 |
| 追踪 | **OpenTelemetry** + **Jaeger** | 分布式链路追踪 |

### 2.3 安全

| 组件 | 选型 | 使用场景 |
|------|------|----------|
| 认证 | **JWT** | 用户认证、token 管理 |
| 授权 | **Casbin** | RBAC/ABAC 权限控制 |
| 传输加密 | **TLS 1.3** | 服务间通信加密 |
| 数据加密 | **AES-256-GCM** | 敏感数据加密存储 |

## 3. Go 依赖包选型

```go
// go.mod 预期依赖

// Web & API
github.com/gin-gonic/gin           // HTTP 框架
google.golang.org/grpc             // gRPC 框架
github.com/grpc-ecosystem/go-grpc-middleware // gRPC 中间件

// 数据存储
go.etcd.io/etcd/client/v3          // etcd 客户端
github.com/dgraph-io/badger/v4     // BadgerDB 嵌入式存储
github.com/jackc/pgx/v5            // PostgreSQL 驱动

// 消息队列
github.com/nats-io/nats.go         // NATS 客户端
github.com/hibiken/asynq           // 异步任务队列

// 配置管理
github.com/spf13/viper             // 配置管理
github.com/spf13/cobra             // CLI 框架

// 可观测性
go.uber.org/zap                    // 结构化日志
github.com/prometheus/client_golang // Prometheus metrics
go.opentelemetry.io/otel           // OpenTelemetry

// 安全
github.com/golang-jwt/jwt/v5       // JWT
github.com/casbin/casbin/v2        // 权限控制

// 工具库
github.com/google/uuid             // UUID 生成
github.com/hashicorp/go-multierror // 错误聚合
golang.org/x/sync                  // 并发原语扩展
```

## 4. 项目目录结构设计

```
JzSE-go/
├── cmd/                          # 可执行入口
│   ├── region/                   # 区域服务启动入口
│   │   └── main.go
│   ├── coordinator/              # 全局协调服务启动入口
│   │   └── main.go
│   └── cli/                      # 命令行工具
│       └── main.go
│
├── internal/                     # 内部包（不对外暴露）
│   ├── region/                   # 区域层实现
│   │   ├── service/             # 区域服务逻辑
│   │   ├── storage/             # 本地存储实现
│   │   ├── metadata/            # 本地元数据管理
│   │   └── sync/                # 同步代理
│   │
│   ├── coordinator/              # 全局协调层实现
│   │   ├── service/             # 协调服务逻辑
│   │   ├── metadata/            # 全局元数据管理
│   │   ├── sync/                # 同步引擎
│   │   └── registry/            # 区域注册管理
│   │
│   └── common/                   # 公共模块
│       ├── config/              # 配置管理
│       ├── logger/              # 日志封装
│       ├── errors/              # 错误定义
│       └── utils/               # 工具函数
│
├── pkg/                          # 可对外暴露的包
│   ├── api/                      # API 定义
│   │   ├── http/                # HTTP API 处理器
│   │   └── grpc/                # gRPC 服务定义
│   │
│   ├── protocol/                 # 协议定义
│   │   ├── proto/               # protobuf 文件
│   │   └── gen/                 # 生成的代码
│   │
│   └── sdk/                      # 客户端 SDK
│
├── configs/                      # 配置文件模板
│   ├── region.yaml
│   └── coordinator.yaml
│
├── deployments/                  # 部署相关
│   ├── docker/
│   └── kubernetes/
│
├── docs/                         # 文档
│   └── architecture/
│
├── scripts/                      # 脚本
│   ├── build.sh
│   └── test.sh
│
├── test/                         # 集成测试
│
├── go.mod
├── go.sum
├── Makefile
├── AGENTS.md
└── README.md
```

## 5. 开发工具链

| 工具 | 用途 |
|------|------|
| `golangci-lint` | 代码静态检查 |
| `mockgen` | Mock 生成 |
| `protoc` + `protoc-gen-go` | protobuf 编译 |
| `swag` | Swagger 文档生成 |
| `air` | 热重载开发 |

## 6. 技术决策记录 (ADR)

### ADR-001: 选择 etcd 作为全局元数据存储

**背景**：需要一个可靠的分布式 KV 存储来管理全局元数据。

**决策**：选择 etcd。

**理由**：
- 强一致性保证 (Raft 协议)
- 成熟的 Go 客户端
- 支持 Watch 机制，适合元数据变更通知
- Kubernetes 生态的核心组件，运维成熟

### ADR-002: 选择 NATS 作为事件总线

**背景**：需要区域间的事件通知机制。

**决策**：选择 NATS。

**理由**：
- 轻量级、高性能
- 支持多种消息模式 (Pub/Sub, Request/Reply, Queue Groups)
- 原生支持集群部署
- Go 原生实现，性能优异

### ADR-003: 采用 BadgerDB 作为本地元数据缓存

**背景**：需要高性能的本地元数据存储。

**决策**：选择 BadgerDB。

**理由**：
- 纯 Go 实现，无 CGO 依赖
- LSM-tree 架构，写性能优异
- 支持事务和 ACID
- Dgraph 背书，活跃维护
