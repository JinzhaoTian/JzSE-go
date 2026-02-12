# 本地开发部署指南

**每个 Region 使用独立存储实例和独立元数据 DB**，默认使用 **MinIO** 作为 Region 的存储后端。

## 前置要求

| 工具 | 版本要求 | 用途 |
|------|----------|------|
| Go | 1.24+ | 编译运行 |
| Make | 任意 | 构建工具 |
| Docker | 20.10+ | 启动 MinIO |
| golangci-lint | (可选) | 代码检查 |

```bash
# 验证 Go 版本
go version
```

## 快速启动（MinIO 推荐路径）

### 1. 安装依赖

```bash
make deps
```

### 2. 启动 MinIO

```bash
docker run -d --name jzse-minio \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v $(pwd)/data/minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"
```

访问 MinIO 控制台：`http://localhost:9001`  
默认账号：`minioadmin` / `minioadmin`

### 3. 启动 Region 服务

`configs/region.yaml` 默认已配置为 MinIO 后端（`storage.backend=minio`）。

```bash
# 使用配置文件启动（推荐）
make run-region
```

服务启动后监听 `http://localhost:8080`。

### 4. 启动 Coordinator 服务

```bash
make run-coordinator
```

服务启动后监听 `http://localhost:8081`。

### 5. 验证服务

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

## 本地多 Region 模拟（每 Region 独立存储实例）

先启动两个 MinIO 实例，分别服务 Beijing 与 Shanghai：

```bash
# Beijing MinIO
docker run -d --name jzse-minio-beijing \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v $(pwd)/data/beijing/minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"

# Shanghai MinIO
docker run -d --name jzse-minio-shanghai \
  -p 9100:9000 -p 9101:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -v $(pwd)/data/shanghai/minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"
```

然后启动两个 Region 实例，分别指向自己的 MinIO 与 metadata 目录：

```bash
# 终端 1 - Beijing Region (port 8080)
JZSE_SERVER_HTTP_ADDR=:8080 \
JZSE_REGION_ID=region-beijing \
JZSE_STORAGE_BACKEND=minio \
JZSE_STORAGE_MINIO_ENDPOINT=localhost:9000 \
JZSE_STORAGE_MINIO_ACCESS_KEY=minioadmin \
JZSE_STORAGE_MINIO_SECRET_KEY=minioadmin \
JZSE_STORAGE_MINIO_BUCKET=jzse-beijing \
JZSE_STORAGE_MINIO_USE_SSL=false \
JZSE_METADATA_DB_PATH=./data/beijing/metadata \
JZSE_LOGGER_DEVELOPMENT=true \
JZSE_LOGGER_FORMAT=console \
  go run ./cmd/region

# 终端 2 - Shanghai Region (port 8082)
JZSE_SERVER_HTTP_ADDR=:8082 \
JZSE_REGION_ID=region-shanghai \
JZSE_STORAGE_BACKEND=minio \
JZSE_STORAGE_MINIO_ENDPOINT=localhost:9100 \
JZSE_STORAGE_MINIO_ACCESS_KEY=minioadmin \
JZSE_STORAGE_MINIO_SECRET_KEY=minioadmin \
JZSE_STORAGE_MINIO_BUCKET=jzse-shanghai \
JZSE_STORAGE_MINIO_USE_SSL=false \
JZSE_METADATA_DB_PATH=./data/shanghai/metadata \
JZSE_LOGGER_DEVELOPMENT=true \
JZSE_LOGGER_FORMAT=console \
  go run ./cmd/region

# 终端 3 - Coordinator (port 8081)
make run-coordinator
```

## 切换存储后端

### 切换到 local_fs（仅本地磁盘）

```bash
JZSE_STORAGE_BACKEND=local_fs \
JZSE_STORAGE_PATH=./data/storage \
JZSE_STORAGE_TEMP_PATH=./data/temp \
  go run ./cmd/region --config configs/region.yaml
```

### 切换到 S3

```bash
JZSE_STORAGE_BACKEND=s3 \
JZSE_STORAGE_S3_ENDPOINT=s3.amazonaws.com \
JZSE_STORAGE_S3_ACCESS_KEY=<your-access-key> \
JZSE_STORAGE_S3_SECRET_KEY=<your-secret-key> \
JZSE_STORAGE_S3_BUCKET=<your-bucket> \
JZSE_STORAGE_S3_REGION=us-east-1 \
JZSE_STORAGE_S3_USE_SSL=true \
  go run ./cmd/region --config configs/region.yaml
```

### 切换到 RustFS（S3 兼容）

```bash
JZSE_STORAGE_BACKEND=rustfs \
JZSE_STORAGE_RUSTFS_ENDPOINT=<rustfs-endpoint> \
JZSE_STORAGE_RUSTFS_ACCESS_KEY=<access-key> \
JZSE_STORAGE_RUSTFS_SECRET_KEY=<secret-key> \
JZSE_STORAGE_RUSTFS_BUCKET=<bucket> \
JZSE_STORAGE_RUSTFS_REGION=us-east-1 \
JZSE_STORAGE_RUSTFS_USE_SSL=false \
  go run ./cmd/region --config configs/region.yaml
```

## 环境变量

所有配置项均可通过环境变量覆盖，前缀为 `JZSE_`，层级用下划线分隔：

| 配置路径 | 环境变量 | 默认值 |
|----------|----------|--------|
| `server.http_addr` | `JZSE_SERVER_HTTP_ADDR` | `:8080` |
| `server.grpc_addr` | `JZSE_SERVER_GRPC_ADDR` | `:9090` |
| `region.id` | `JZSE_REGION_ID` | `region-default` |
| `storage.backend` | `JZSE_STORAGE_BACKEND` | `minio`（来自 `configs/region.yaml`） |
| `storage.path` | `JZSE_STORAGE_PATH` | `./data/storage` |
| `storage.minio.endpoint` | `JZSE_STORAGE_MINIO_ENDPOINT` | `localhost:9000` |
| `storage.minio.access_key` | `JZSE_STORAGE_MINIO_ACCESS_KEY` | `minioadmin` |
| `storage.minio.secret_key` | `JZSE_STORAGE_MINIO_SECRET_KEY` | `minioadmin` |
| `storage.minio.bucket` | `JZSE_STORAGE_MINIO_BUCKET` | `jzse` |
| `storage.minio.prefix` | `JZSE_STORAGE_MINIO_PREFIX` | `""` |
| `metadata.db_path` | `JZSE_METADATA_DB_PATH` | `./data/metadata` |
| `sync.mode` | `JZSE_SYNC_MODE` | `push` |
| `logger.level` | `JZSE_LOGGER_LEVEL` | `info` |
| `logger.format` | `JZSE_LOGGER_FORMAT` | `json` |
| `logger.development` | `JZSE_LOGGER_DEVELOPMENT` | `false` |

## 数据目录

本地运行时，默认使用目录如下：

```text
data/
├── beijing/
│   ├── metadata/   # Beijing Region BadgerDB
│   └── minio/      # Beijing Region MinIO 数据
├── shanghai/
│   ├── metadata/   # Shanghai Region BadgerDB
│   └── minio/      # Shanghai Region MinIO 数据
└── temp/           # 上传临时文件
```

如果切换到 `local_fs`，会额外使用：

```text
data/storage/    # 文件内容（local_fs 后端）
```

清理数据：

```bash
rm -rf ./data
```

停止并删除本地 MinIO 容器：

```bash
docker rm -f jzse-minio jzse-minio-beijing jzse-minio-shanghai
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
