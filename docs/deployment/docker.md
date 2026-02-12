# Docker Compose 部署指南

适用于单机部署、测试环境和开发联调，默认使用 **MinIO** 作为 Region 存储后端。
跨 Region 部署时，建议每个 Region 绑定独立 MinIO 与独立 metadata volume，避免跨区域共享状态。

## 前置要求

| 工具 | 版本要求 |
|------|----------|
| Docker | 20.10+ |
| Docker Compose | v2.0+ |

## 架构（推荐：MinIO）

```text
docker-compose 启动以下容器：

  etcd (2379)  ←──┐
  nats (4222)  ←──┤
  minio-beijing (9000)  ←┤
  minio-shanghai (9100) ←┤
                  │
  coordinator (8081/9091)
       ↑
  region-beijing  (8080/9090)
  region-shanghai (8082/9092)
```

## 快速启动（MinIO 推荐路径）

### 1. 进入部署目录

```bash
cd deployments/docker
```

### 2. 创建 MinIO override 文件

创建 `docker-compose.minio.yaml`：

```yaml
services:
  minio-beijing:
    image: quay.io/minio/minio:latest
    container_name: jzse-minio-beijing
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio-beijing-data:/data

  minio-shanghai:
    image: quay.io/minio/minio:latest
    container_name: jzse-minio-shanghai
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9100:9000"
      - "9101:9001"
    volumes:
      - minio-shanghai-data:/data

  region-beijing:
    environment:
      JZSE_STORAGE_BACKEND: "minio"
      JZSE_STORAGE_MINIO_ENDPOINT: "minio-beijing:9000"
      JZSE_STORAGE_MINIO_ACCESS_KEY: "minioadmin"
      JZSE_STORAGE_MINIO_SECRET_KEY: "minioadmin"
      JZSE_STORAGE_MINIO_BUCKET: "jzse-beijing"
      JZSE_STORAGE_MINIO_USE_SSL: "false"
    depends_on:
      - minio-beijing

  region-shanghai:
    environment:
      JZSE_STORAGE_BACKEND: "minio"
      JZSE_STORAGE_MINIO_ENDPOINT: "minio-shanghai:9000"
      JZSE_STORAGE_MINIO_ACCESS_KEY: "minioadmin"
      JZSE_STORAGE_MINIO_SECRET_KEY: "minioadmin"
      JZSE_STORAGE_MINIO_BUCKET: "jzse-shanghai"
      JZSE_STORAGE_MINIO_USE_SSL: "false"
    depends_on:
      - minio-shanghai

volumes:
  minio-beijing-data:
  minio-shanghai-data:
```

### 3. 启动全部服务

```bash
docker compose -f docker-compose.yaml -f docker-compose.minio.yaml up -d --build

# 查看日志
docker compose -f docker-compose.yaml -f docker-compose.minio.yaml logs -f

# 查看服务状态
docker compose -f docker-compose.yaml -f docker-compose.minio.yaml ps
```

MinIO 控制台：
- Beijing: `http://localhost:9001`
- Shanghai: `http://localhost:9101`

## 验证服务

```bash
# Coordinator 健康检查
curl http://localhost:8081/api/v1/health

# Beijing Region 健康检查
curl http://localhost:8080/api/v1/health

# Shanghai Region 健康检查
curl http://localhost:8082/api/v1/health

# 上传文件到 Beijing Region
curl -X POST http://localhost:8080/api/v1/files \
  -F "file=@test.txt" \
  -F "path=/docs/test.txt"
```

## 服务端口映射

| 服务 | HTTP 端口 | 其他端口 | 说明 |
|------|-----------|----------|------|
| coordinator | 8081 | 9091(gRPC) | 全局协调服务 |
| region-beijing | 8080 | 9090(gRPC) | 北京区域 |
| region-shanghai | 8082 | 9092(gRPC) | 上海区域 |
| etcd | 2379 | - | 元数据存储 |
| nats | 4222 | 8222(监控) | 消息队列 |
| minio-beijing | 9000 | 9001(控制台) | 北京区域对象存储 |
| minio-shanghai | 9100 | 9101(控制台) | 上海区域对象存储 |

## 切换存储后端

### 切换到 local_fs

直接只使用基础文件启动（不带 override）：

```bash
docker compose up -d --build
```

`local_fs` 会使用 `beijing-storage` / `shanghai-storage` volume 持久化。

### 切换到 S3

在 override 文件中给 Region 增加：

```yaml
environment:
  JZSE_STORAGE_BACKEND: "s3"
  JZSE_STORAGE_S3_ENDPOINT: "s3.amazonaws.com"
  JZSE_STORAGE_S3_ACCESS_KEY: "<your-access-key>"
  JZSE_STORAGE_S3_SECRET_KEY: "<your-secret-key>"
  JZSE_STORAGE_S3_BUCKET: "<your-bucket>"
  JZSE_STORAGE_S3_REGION: "us-east-1"
  JZSE_STORAGE_S3_USE_SSL: "true"
```

### 切换到 RustFS（S3 兼容）

在 override 文件中给 Region 增加：

```yaml
environment:
  JZSE_STORAGE_BACKEND: "rustfs"
  JZSE_STORAGE_RUSTFS_ENDPOINT: "<rustfs-endpoint>"
  JZSE_STORAGE_RUSTFS_ACCESS_KEY: "<access-key>"
  JZSE_STORAGE_RUSTFS_SECRET_KEY: "<secret-key>"
  JZSE_STORAGE_RUSTFS_BUCKET: "<bucket>"
  JZSE_STORAGE_RUSTFS_REGION: "us-east-1"
  JZSE_STORAGE_RUSTFS_USE_SSL: "false"
```

## 常用操作

```bash
# 停止全部服务（MinIO 模式）
docker compose -f docker-compose.yaml -f docker-compose.minio.yaml down

# 停止并清除数据卷（MinIO 模式）
docker compose -f docker-compose.yaml -f docker-compose.minio.yaml down -v

# 查看某个服务日志
docker compose -f docker-compose.yaml -f docker-compose.minio.yaml logs -f region-beijing

# 进入容器调试
docker compose -f docker-compose.yaml -f docker-compose.minio.yaml exec region-beijing sh
```

## 扩展 Region

在 override 文件或 `docker-compose.yaml` 中添加新的 Region 服务，关键环境变量建议包含：

```yaml
environment:
  JZSE_REGION_ID: "region-shenzhen"
  JZSE_REGION_NAME: "Shenzhen Region"
  JZSE_REGION_LOCATION: "shenzhen"
  JZSE_COORDINATOR_ENDPOINTS: "coordinator:8081"
  JZSE_STORAGE_BACKEND: "minio"
  JZSE_STORAGE_MINIO_ENDPOINT: "minio-shenzhen:9000"
  JZSE_STORAGE_MINIO_ACCESS_KEY: "minioadmin"
  JZSE_STORAGE_MINIO_SECRET_KEY: "minioadmin"
  JZSE_STORAGE_MINIO_BUCKET: "jzse-shenzhen"
```

同时新增 `minio-shenzhen` 服务与 `minio-shenzhen-data` 卷，保证 Shenzhen Region 使用独立对象存储实例。

## 数据持久化

| Volume | 说明 |
|--------|------|
| `etcd-data` | etcd 数据 |
| `beijing-metadata` | 北京区域元数据 (BadgerDB) |
| `shanghai-metadata` | 上海区域元数据 (BadgerDB) |
| `beijing-storage` | 北京区域 local_fs 文件存储（仅 local_fs） |
| `shanghai-storage` | 上海区域 local_fs 文件存储（仅 local_fs） |
| `minio-beijing-data` | Beijing 独立 MinIO 对象数据 |
| `minio-shanghai-data` | Shanghai 独立 MinIO 对象数据 |

## 单独构建镜像

不使用 Compose 时，也可从项目根目录直接构建：

```bash
# 从项目根目录
docker build -t jzse-region:latest -f deployments/docker/Dockerfile.region .
docker build -t jzse-coordinator:latest -f deployments/docker/Dockerfile.coordinator .
```

或使用 Makefile：

```bash
make docker-build
```
