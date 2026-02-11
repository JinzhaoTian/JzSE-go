# 本地开发部署指南

## 前置要求

| 工具 | 版本要求 | 用途 |
|------|----------|------|
| Go | 1.24+ | 编译运行 |
| Make | 任意 | 构建工具 |
| golangci-lint | (可选) | 代码检查 |

```bash
# 验证 Go 版本
go version
```

## 快速启动

### 1. 安装依赖

```bash
make deps
```

### 2. 启动 Region 服务

Region 服务仅依赖 BadgerDB（嵌入式）和本地文件系统，无需外部服务。

```bash
# 开发模式（console 日志，便于调试）
make dev-region

# 或使用配置文件启动
make run-region
```

服务启动后监听 `http://localhost:8080`。

### 3. 启动 Coordinator 服务

Coordinator 当前使用内存实现（etcd/NATS 集成 TODO），可直接运行。

```bash
make run-coordinator
```

服务启动后监听 `http://localhost:8081`。

### 4. 验证服务

```bash
# Region 健康检查
curl http://localhost:8080/api/v1/health

# Coordinator 健康检查
curl http://localhost:8081/api/v1/health
```

## 功能测试

### 文件操作

```bash
# 上传文件
curl -X POST http://localhost:8080/api/v1/files \
  -F "file=@test.txt" \
  -F "path=/docs/test.txt"

# 下载文件（替换 <file_id> 为上传返回的 ID）
curl http://localhost:8080/api/v1/files/<file_id> -o downloaded.txt

# 查看文件元数据
curl http://localhost:8080/api/v1/files/<file_id>/metadata

# 列出目录内容
curl http://localhost:8080/api/v1/directories/docs

# 删除文件
curl -X DELETE http://localhost:8080/api/v1/files/<file_id>
```

### Coordinator 操作

```bash
# 查看所有区域
curl http://localhost:8081/api/v1/regions

# 查看指定区域
curl http://localhost:8081/api/v1/regions/region-beijing
```

## 本地多 Region 模拟

通过环境变量在同一台机器上启动多个 Region 实例：

```bash
# 终端 1 - Beijing Region (port 8080)
JZSE_SERVER_HTTP_ADDR=:8080 \
JZSE_REGION_ID=region-beijing \
JZSE_STORAGE_PATH=./data/beijing/storage \
JZSE_METADATA_DB_PATH=./data/beijing/metadata \
JZSE_LOGGER_DEVELOPMENT=true \
JZSE_LOGGER_FORMAT=console \
  go run ./cmd/region

# 终端 2 - Shanghai Region (port 8082)
JZSE_SERVER_HTTP_ADDR=:8082 \
JZSE_REGION_ID=region-shanghai \
JZSE_STORAGE_PATH=./data/shanghai/storage \
JZSE_METADATA_DB_PATH=./data/shanghai/metadata \
JZSE_LOGGER_DEVELOPMENT=true \
JZSE_LOGGER_FORMAT=console \
  go run ./cmd/region

# 终端 3 - Coordinator (port 8081)
make run-coordinator
```

## 运行测试

```bash
# 全量测试（竞态检测 + 覆盖率）
make test

# 生成 HTML 覆盖率报告
make test-coverage
open coverage.html

# 代码质量检查
make lint
make vet
make fmt
```

## 构建二进制

```bash
# 构建全部
make build

# 单独构建
make build-region
make build-coordinator

# 运行二进制
./bin/region --config configs/region.yaml
./bin/coordinator --config configs/coordinator.yaml
```

## 环境变量

所有配置项均可通过环境变量覆盖，前缀为 `JZSE_`，层级用下划线分隔：

| 配置路径 | 环境变量 | 默认值 |
|----------|----------|--------|
| `server.http_addr` | `JZSE_SERVER_HTTP_ADDR` | `:8080` |
| `server.grpc_addr` | `JZSE_SERVER_GRPC_ADDR` | `:9090` |
| `region.id` | `JZSE_REGION_ID` | `region-default` |
| `storage.path` | `JZSE_STORAGE_PATH` | `./data/storage` |
| `metadata.db_path` | `JZSE_METADATA_DB_PATH` | `./data/metadata` |
| `sync.mode` | `JZSE_SYNC_MODE` | `push` |
| `logger.level` | `JZSE_LOGGER_LEVEL` | `info` |
| `logger.format` | `JZSE_LOGGER_FORMAT` | `json` |
| `logger.development` | `JZSE_LOGGER_DEVELOPMENT` | `false` |

## 数据目录

本地运行时，数据默认存储在项目根目录下的 `./data/`：

```
data/
├── storage/     # 文件内容（本地文件系统后端）
├── metadata/    # BadgerDB 元数据
└── temp/        # 上传临时文件
```

清理数据：

```bash
rm -rf ./data
```
