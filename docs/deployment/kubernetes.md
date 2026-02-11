# Kubernetes 生产部署指南

适用于生产环境的多区域分布式部署。

## 前置要求

| 工具 | 版本要求 | 用途 |
|------|----------|------|
| kubectl | 1.28+ | 集群管理 |
| Docker | 20.10+ | 镜像构建 |
| Helm | 3.x (可选) | etcd/NATS Operator 安装 |

## 架构概览

```
Ingress (TLS)
    │
    ├── beijing.jzse.example.com  → Service/region-beijing  → Deployment/region-beijing
    ├── shanghai.jzse.example.com → Service/region-shanghai → Deployment/region-shanghai
    └── coordinator.jzse.example.com → Service/coordinator  → StatefulSet/coordinator
                                                                    │
                                                            ┌───────┴───────┐
                                                            │               │
                                                        etcd cluster    NATS cluster
```

## 资源清单

| 文件 | 资源类型 | 说明 |
|------|----------|------|
| `namespace.yaml` | Namespace | `jzse` 命名空间 |
| `configmap-region.yaml` | ConfigMap | Region 配置 |
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

etcd 和 NATS 建议使用 Operator 部署，确保高可用：

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

### 3. 部署 JzSE 服务

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

### 4. 验证部署

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
   ```
4. 修改 PVC 名称为 `region-shanghai-data`
5. 在 Ingress 中添加对应的 host 规则

```bash
kubectl apply -f region-shanghai.yaml
```

## 生产环境建议

### 存储

- Region PVC 使用 SSD StorageClass 以保证 BadgerDB 和文件 I/O 性能
- 根据数据量规划 PVC 大小，建议起步 50Gi

```yaml
storageClassName: ssd   # 替换为集群实际的 SSD StorageClass
```

### 高可用

- Coordinator：至少 2 副本 + Leader 选举（待实现）
- Region：单实例即可（每个 Region 独立运作），通过多 Region 实现全局高可用
- etcd：3 或 5 节点集群
- NATS：3 节点集群 + JetStream

### 资源配额

根据实际负载调整，以下为参考值：

| 组件 | CPU Request | CPU Limit | Memory Request | Memory Limit |
|------|-------------|-----------|----------------|--------------|
| Coordinator | 100m | 500m | 128Mi | 512Mi |
| Region | 100m | 1000m | 128Mi | 1Gi |

### 网络与安全

- 前端流量通过 Ingress + TLS 加密
- Region 与 Coordinator 之间使用 gRPC，生产环境建议配置 mTLS
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
  ```yaml
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
  ```
- 日志：JSON 格式输出到 stdout，由 Fluentd/Loki 采集
- 推荐 Grafana 面板监控关键指标：请求延迟、文件操作吞吐、同步延迟、Region 健康状态

### CI/CD

推荐流水线：

```
代码提交 → GitHub Actions (lint + test + build image) → 推送 Registry → ArgoCD 自动同步部署
```
