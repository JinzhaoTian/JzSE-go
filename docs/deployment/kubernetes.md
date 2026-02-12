# Kubernetes 生产部署指南

适用于生产环境的多区域分布式部署。  
Region 存储后端默认推荐 **MinIO（或其他 S3 兼容对象存储）**。
跨 Region 部署时，建议每个 Region 使用独立存储实例与独立 metadata PVC。

## 前置要求

| 工具 | 版本要求 | 用途 |
|------|----------|------|
| kubectl | 1.28+ | 集群管理 |
| Docker | 20.10+ | 镜像构建 |
| Helm | 3.x (可选) | etcd/NATS/MinIO 安装 |

## 架构概览（MinIO 推荐）

```text
Ingress (TLS)
    │
    ├── beijing.jzse.example.com  → Service/region-beijing  → Deployment/region-beijing
    ├── shanghai.jzse.example.com → Service/region-shanghai → Deployment/region-shanghai
    └── coordinator.jzse.example.com → Service/coordinator  → StatefulSet/coordinator
                                                                    │
                                                            ┌───────┴───────┐
                                                            │               │
                                                        etcd cluster    NATS cluster
                                                            │
                                         region-beijing → minio-beijing (or s3-beijing)
                                         region-shanghai → minio-shanghai (or s3-shanghai)
```

## 资源清单

| 文件 | 资源类型 | 说明 |
|------|----------|------|
| `namespace.yaml` | Namespace | `jzse` 命名空间 |
| `configmap-region.yaml` | ConfigMap | Region 基础配置 |
| `configmap-coordinator.yaml` | ConfigMap | Coordinator 配置 |
| `coordinator.yaml` | StatefulSet + Service | Coordinator 服务 |
| `region.yaml` | Deployment + PVC + Service | Region 服务 (Beijing 示例) |
| `ingress.yaml` | Ingress | 外部流量入口 |

## 部署步骤

### 1. 构建并推送镜像

```bash
# 构建镜像
make docker-build

# 打标签并推送到 Registry（替换为你的 Registry 地址）
docker tag jzse-region:latest your-registry.com/jzse-region:v0.1.0
docker tag jzse-coordinator:latest your-registry.com/jzse-coordinator:v0.1.0
docker push your-registry.com/jzse-region:v0.1.0
docker push your-registry.com/jzse-coordinator:v0.1.0
```

### 2. 部署基础设施

etcd 和 NATS 建议使用 Operator/Helm 部署，确保高可用：

```bash
# etcd (使用 Bitnami Helm Chart)
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install etcd bitnami/etcd \
  --namespace jzse --create-namespace \
  --set replicaCount=3 \
  --set auth.rbac.create=false

# NATS (使用官方 Helm Chart)
helm repo add nats https://nats-io.github.io/k8s/helm/charts/
helm install nats nats/nats \
  --namespace jzse \
  --set nats.jetstream.enabled=true
```

MinIO 可使用各区域独立实例或各区域独立对象存储账号，确保 Region 只访问本区域对象存储 Endpoint。

### 3. 配置 Region 使用 MinIO（推荐）

1. 为每个 Region 创建独立存储密钥：

```bash
kubectl create secret generic region-beijing-storage-secret -n jzse \
  --from-literal=MINIO_ACCESS_KEY=minioadmin \
  --from-literal=MINIO_SECRET_KEY=minioadmin

kubectl create secret generic region-shanghai-storage-secret -n jzse \
  --from-literal=MINIO_ACCESS_KEY=minioadmin \
  --from-literal=MINIO_SECRET_KEY=minioadmin
```

2. 按 Region 修改对应 `region-*.yaml`，确保每个 Region 指向自己的 MinIO Endpoint 与 Secret。  
以下示例为 Beijing Region：

```yaml
- name: JZSE_STORAGE_BACKEND
  value: "minio"
- name: JZSE_STORAGE_MINIO_ENDPOINT
  value: "minio-beijing.jzse.svc.cluster.local:9000"
- name: JZSE_STORAGE_MINIO_BUCKET
  value: "jzse-beijing"
- name: JZSE_STORAGE_MINIO_REGION
  value: "us-east-1"
- name: JZSE_STORAGE_MINIO_USE_SSL
  value: "false"
- name: JZSE_STORAGE_MINIO_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: region-beijing-storage-secret
      key: MINIO_ACCESS_KEY
- name: JZSE_STORAGE_MINIO_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: region-beijing-storage-secret
      key: MINIO_SECRET_KEY
```

说明：对象数据写入各自 MinIO 后，Region PVC 主要用于该 Region 的 `metadata.db_path`（BadgerDB）和临时目录。  
不要在多个 Region 之间共享同一个 metadata PVC 或同一套存储凭据。

### 4. 部署 JzSE 服务

```bash
cd deployments/kubernetes

# 创建命名空间
kubectl apply -f namespace.yaml

# 创建配置
kubectl apply -f configmap-region.yaml
kubectl apply -f configmap-coordinator.yaml

# 部署 Coordinator
kubectl apply -f coordinator.yaml

# 部署 Region
kubectl apply -f region.yaml

# 配置 Ingress
kubectl apply -f ingress.yaml
```

### 5. 验证部署

```bash
# 查看 Pod 状态
kubectl get pods -n jzse

# 查看服务
kubectl get svc -n jzse

# 检查 Coordinator 健康
kubectl port-forward svc/coordinator 8081:8081 -n jzse
curl http://localhost:8081/api/v1/health

# 检查 Region 健康
kubectl port-forward svc/region-beijing 8080:8080 -n jzse
curl http://localhost:8080/api/v1/health
```

## 添加新 Region

以添加上海区域为例，复制 `region.yaml` 并修改：

1. 修改 `metadata.name` 为 `region-shanghai`
2. 修改 `app.kubernetes.io/instance` 标签为 `region-shanghai`
3. 修改环境变量：

```yaml
env:
  - name: JZSE_REGION_ID
    value: "region-shanghai"
  - name: JZSE_REGION_NAME
    value: "Shanghai Region"
  - name: JZSE_REGION_LOCATION
    value: "shanghai"
  - name: JZSE_STORAGE_MINIO_ENDPOINT
    value: "minio-shanghai.jzse.svc.cluster.local:9000"
  - name: JZSE_STORAGE_MINIO_BUCKET
    value: "jzse-shanghai"
```

4. 绑定上海区域独立 Secret（如 `region-shanghai-storage-secret`）
5. 修改 PVC 名称为 `region-shanghai-data`
6. 在 Ingress 中添加对应的 host 规则

```bash
kubectl create secret generic region-shanghai-storage-secret -n jzse \
  --from-literal=MINIO_ACCESS_KEY=<shanghai-access-key> \
  --from-literal=MINIO_SECRET_KEY=<shanghai-secret-key>
```

```bash
kubectl apply -f region-shanghai.yaml
```

## 切换存储后端

### 切换到 local_fs

- 设置 `JZSE_STORAGE_BACKEND=local_fs`
- 在 `configmap-region.yaml` 保持：
  - `storage.path: /data/storage`
  - `storage.temp_path: /data/temp`
- 每个 Region 保留自己独立 PVC（文件内容 + 元数据均落盘）

### 切换到 S3

在 Region `env` 中设置：

```yaml
- name: JZSE_STORAGE_BACKEND
  value: "s3"
- name: JZSE_STORAGE_S3_ENDPOINT
  value: "s3.amazonaws.com"
- name: JZSE_STORAGE_S3_BUCKET
  value: "<your-bucket>"
- name: JZSE_STORAGE_S3_REGION
  value: "us-east-1"
- name: JZSE_STORAGE_S3_USE_SSL
  value: "true"
- name: JZSE_STORAGE_S3_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: region-<id>-storage-secret
      key: S3_ACCESS_KEY
- name: JZSE_STORAGE_S3_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: region-<id>-storage-secret
      key: S3_SECRET_KEY
```

### 切换到 RustFS（S3 兼容）

在 Region `env` 中设置：

```yaml
- name: JZSE_STORAGE_BACKEND
  value: "rustfs"
- name: JZSE_STORAGE_RUSTFS_ENDPOINT
  value: "<rustfs-endpoint>"
- name: JZSE_STORAGE_RUSTFS_BUCKET
  value: "<bucket>"
- name: JZSE_STORAGE_RUSTFS_REGION
  value: "us-east-1"
- name: JZSE_STORAGE_RUSTFS_USE_SSL
  value: "false"
- name: JZSE_STORAGE_RUSTFS_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: region-<id>-storage-secret
      key: RUSTFS_ACCESS_KEY
- name: JZSE_STORAGE_RUSTFS_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: region-<id>-storage-secret
      key: RUSTFS_SECRET_KEY
```

## 生产环境建议

### 存储

- 推荐每个 Region 使用独立 MinIO 集群/租户或独立 S3 账号（至少独立 bucket + 凭据）
- Region PVC 使用 SSD StorageClass 存储该 Region 的 BadgerDB 元数据
- 根据数据量规划 PVC 大小，建议起步 50Gi

```yaml
storageClassName: ssd   # 替换为集群实际的 SSD StorageClass
```

### 高可用

- Coordinator：至少 2 副本 + Leader 选举（待实现）
- Region：每个 Region 1 实例起步，跨 Region 实现全局高可用
- etcd：3 或 5 节点集群
- NATS：3 节点集群 + JetStream
- MinIO/S3：按对象存储系统自身 HA 方案部署，且按 Region 做故障域隔离

### 资源配额

根据实际负载调整，以下为参考值：

| 组件 | CPU Request | CPU Limit | Memory Request | Memory Limit |
|------|-------------|-----------|----------------|--------------|
| Coordinator | 100m | 500m | 128Mi | 512Mi |
| Region | 100m | 1000m | 128Mi | 1Gi |

### 网络与安全

- 前端流量通过 Ingress + TLS 加密
- Region 与 Coordinator 之间使用 gRPC，生产环境建议配置 mTLS
- 对象存储访问密钥放入 Kubernetes Secret，不写入 ConfigMap
- 使用 NetworkPolicy 限制 Pod 间通信范围

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: region-network-policy
  namespace: jzse
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: region
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: jzse
      ports:
        - port: 8080
        - port: 9090
```

### 监控

- 接入 Prometheus：在 Pod 上添加 annotation 以自动发现
- 日志：JSON 格式输出到 stdout，由 Fluentd/Loki 采集
- 推荐 Grafana 面板监控关键指标：请求延迟、文件操作吞吐、同步延迟、Region 健康状态

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### CI/CD

推荐流水线：

```text
代码提交 → GitHub Actions (lint + test + build image) → 推送 Registry → ArgoCD 自动同步部署
```
