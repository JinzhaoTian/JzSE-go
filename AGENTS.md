# AGENTS.md - JzSE 开发指南

## 项目概述

JzSE (Jinzhao's Storage Engine) 是一个多区域协同文件服务系统，采用 Go 语言开发。

## 架构快速参考

```
JzSE-go/
├── cmd/               # 入口点
│   ├── region/       # 区域服务
│   ├── coordinator/  # 全局协调器
│   └── cli/          # 命令行工具
├── internal/          # 内部实现
│   ├── region/       # 区域层
│   ├── coordinator/  # 协调层
│   └── common/       # 公共模块
├── pkg/               # 公开包
│   ├── api/          # API 定义
│   ├── protocol/     # protobuf
│   └── sdk/          # 客户端SDK
├── configs/           # 配置模板
├── docs/              # 文档
└── deployments/       # 部署配置
```

## 核心模块

| 模块 | 路径 | 职责 |
|------|------|------|
| Region Service | `internal/region/service` | 处理本地文件读写 |
| Local Storage | `internal/region/storage` | 文件存储抽象 |
| Local Metadata | `internal/region/metadata` | 本地元数据管理 |
| Sync Agent | `internal/region/sync` | 同步代理 |
| Global Metadata | `internal/coordinator/metadata` | 全局元数据 |
| Sync Engine | `internal/coordinator/sync` | 同步引擎 |
| Conflict Resolver | `internal/coordinator/conflict` | 冲突解决 |

## 开发规范

### 代码风格

- 遵循 [Effective Go](https://golang.org/doc/effective_go)
- 使用 `golangci-lint` 进行代码检查
- 接口定义在独立文件，实现在同目录下

### 命名规范

```go
// 接口以 -er 结尾或描述能力
type StorageBackend interface { ... }
type ConflictResolver interface { ... }

// 结构体使用名词
type FileMetadata struct { ... }
type RegionInfo struct { ... }

// 方法使用动词
func (s *Service) HandleUpload(...) { ... }
func (r *Resolver) Resolve(...) { ... }
```

### 错误处理

```go
// 定义错误类型在 internal/common/errors
var (
    ErrNotFound     = errors.New("not found")
    ErrConflict     = errors.New("conflict detected")
    ErrSyncFailed   = errors.New("sync failed")
)

// 使用 errors.Wrap 添加上下文
return errors.Wrap(err, "failed to save metadata")
```

### 日志规范

```go
import "go.uber.org/zap"

logger.Info("file uploaded",
    zap.String("file_id", fileID),
    zap.Int64("size", size),
)

logger.Error("sync failed",
    zap.Error(err),
    zap.String("region_id", regionID),
)
```

## 常用命令

```bash
# 构建
make build

# 运行测试
make test

# 代码检查
make lint

# 生成 protobuf
make proto

# 启动区域服务
go run cmd/region/main.go --config configs/region.yaml

# 启动协调器
go run cmd/coordinator/main.go --config configs/coordinator.yaml
```

## 测试规范

- 单元测试: `*_test.go` 在同目录
- 集成测试: `test/` 目录
- Mock 使用 `mockgen` 生成

```go
// 表驱动测试
func TestFileMetadata_Validate(t *testing.T) {
    tests := []struct {
        name    string
        meta    *FileMetadata
        wantErr bool
    }{
        {"valid", &FileMetadata{...}, false},
        {"empty_name", &FileMetadata{Name: ""}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.meta.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## 详细文档

- [架构概览](docs/architecture/overview.md)
- [技术选型](docs/architecture/tech-stack.md)
- [区域层设计](docs/architecture/region-layer.md)
- [全局协调层设计](docs/architecture/global-layer.md)
- [同步机制设计](docs/architecture/sync-mechanism.md)
