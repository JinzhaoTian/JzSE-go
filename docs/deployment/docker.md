# Docker Compose 部署指南

适用于单机部署、测试环境和开发联调。

## 前置要求

| 工具 | 版本要求 |
|------|----------|
| Docker | 20.10+ |
| Docker Compose | v2.0+ |

## 架构

```
docker-compose 启动以下容器：

  etcd (2379)  ←──┐
  nats (4222)  ←──┤
                  │
  coordinator (8081/9091)
       ↑
  region-beijing  (8080/9090)
  region-shanghai (8082/9092)
```

## 快速启动

```bash
cd deployments/docker

# 构建并启动全部服务
docker compose up -d --build

# 查看日志
docker compose logs -f

# 查看服务状态
docker compose ps
```

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

| 服务 | HTTP 端口 | gRPC 端口 | 说明 |
|------|-----------|-----------|------|
| coordinator | 8081 | 9091 | 全局协调服务 |
| region-beijing | 8080 | 9090 | 北京区域 |
| region-shanghai | 8082 | 9092 | 上海区域 |
| etcd | 2379 | - | 元数据存储 |
| nats | 4222 | 8222 (监控) | 消息队列 |

## 常用操作

```bash
# 停止全部服务
docker compose down

# 停止并清除数据卷
docker compose down -v

# 重新构建单个服务
docker compose build region-beijing
docker compose up -d region-beijing

# 查看某个服务日志
docker compose logs -f region-beijing

# 进入容器调试
docker compose exec region-beijing sh
```

## 扩展 Region

在 `docker-compose.yaml` 中添加新的 Region 服务：

```yaml
region-shenzhen:
  build:
    context: ../..
    dockerfile: deployments/docker/Dockerfile.region
  ports:
    - "8083:8080"
    - "9093:9090"
  environment:
    JZSE_REGION_ID: "region-shenzhen"
    JZSE_REGION_NAME: "Shenzhen Region"
    JZSE_REGION_LOCATION: "shenzhen"
    JZSE_COORDINATOR_ENDPOINTS: "coordinator:8081"
  volumes:
    - shenzhen-storage:/data/storage
    - shenzhen-metadata:/data/metadata
  depends_on:
    - coordinator
  restart: unless-stopped
```

并在 `volumes` 部分添加：

```yaml
volumes:
  shenzhen-storage:
  shenzhen-metadata:
```

## 数据持久化

各服务数据通过 Docker Volume 持久化：

| Volume | 说明 |
|--------|------|
| `etcd-data` | etcd 数据 |
| `beijing-storage` | 北京区域文件存储 |
| `beijing-metadata` | 北京区域元数据 (BadgerDB) |
| `shanghai-storage` | 上海区域文件存储 |
| `shanghai-metadata` | 上海区域元数据 (BadgerDB) |

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
