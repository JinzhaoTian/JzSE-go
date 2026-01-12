# JzSE-go

**JzSE** (Jinzhao's Storage Engine) - 多区域协同文件服务系统

## 概述

JzSE 是一个从零设计的分布式文件服务系统，专注于：

- 🚀 **本地高性能访问** - 用户在本地区域享受低延迟文件操作
- 🌍 **全局数据一致性** - 跨区域数据通过异步同步保持最终一致
- 🏠 **区域自治** - 各区域可独立运作，故障隔离

## 架构

系统分为两个核心层次：

| 层次 | 部署位置 | 职责 |
|------|----------|------|
| **区域层** | 每个地理区域 | 直接服务用户读写请求 |
| **全局协调层** | 核心数据中心 | 元数据管理、跨区域同步协调 |

```
         ┌─────────┐   ┌─────────┐   ┌─────────┐
         │ Region  │   │ Region  │   │ Region  │
         │ Beijing │   │Shanghai │   │Shenzhen │
         └────┬────┘   └────┬────┘   └────┬────┘
              │             │             │
              └─────────────┼─────────────┘
                            │
                 ┌──────────▼──────────┐
                 │ Global Coordinator  │
                 └─────────────────────┘
```

## 技术栈

- **语言**: Go 1.21+
- **Web框架**: Gin (HTTP) + gRPC
- **元数据存储**: etcd + BadgerDB
- **消息队列**: NATS
- **可观测性**: Prometheus + Zap

## 快速开始

```bash
# 克隆项目
git clone https://github.com/your-repo/JzSE-go.git
cd JzSE-go

# 安装依赖
go mod download

# 启动区域服务
go run cmd/region/main.go --config configs/region.yaml

# 启动协调服务
go run cmd/coordinator/main.go --config configs/coordinator.yaml
```

## 项目结构

```
JzSE-go/
├── cmd/                # 可执行入口
├── internal/           # 内部实现
├── pkg/                # 公开包
├── configs/            # 配置模板
├── docs/               # 设计文档
├── deployments/        # 部署配置
└── test/               # 集成测试
```

## 文档

- [架构概览](docs/architecture/overview.md)
- [技术选型](docs/architecture/tech-stack.md)
- [区域层设计](docs/architecture/region-layer.md)
- [全局协调层设计](docs/architecture/global-layer.md)
- [同步机制设计](docs/architecture/sync-mechanism.md)
- [开发指南](AGENTS.md)

## 开发状态

🚧 **开发中** - 当前处于架构设计阶段

## License

MIT License
