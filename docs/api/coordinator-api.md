# Coordinator Service API 文档

**Base URL**: `http://localhost:8081/api/v1`

---

## 健康检查

### GET /health

检查 Coordinator 服务健康状态。

**响应**

```
Status: 200 OK
```

```json
{
  "status": "healthy"
}
```

---

## 元数据操作

### GET /metadata/:id

获取文件的全局元数据，包括跨区域存储位置信息。

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | string (path) | 文件 ID (UUID) |

**响应**

```
Status: 200 OK
```

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "test.txt",
  "path": "/docs/test.txt",
  "size": 1024,
  "content_hash": "e3b0c44298fc1c149afbf4c8996fb924...",
  "mime_type": "text/plain",
  "version": 2,
  "vector_clock": {
    "region-beijing": 1,
    "region-shanghai": 1
  },
  "owner_id": "anonymous",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T11:00:00Z",
  "created_by": "anonymous",
  "updated_by": "anonymous",
  "origin_region": "region-beijing",
  "local_state": "present",
  "sync_state": "synced",
  "locations": [
    {
      "region_id": "region-beijing",
      "state": "synced",
      "last_sync_at": "2025-01-15T11:00:00Z"
    },
    {
      "region_id": "region-shanghai",
      "state": "syncing",
      "last_sync_at": "2025-01-15T10:55:00Z"
    }
  ],
  "primary": "region-beijing",
  "replicas": 2
}
```

**全局元数据扩展字段**

| 字段 | 说明 |
|------|------|
| `locations` | 文件在各区域的存储状态列表 |
| `locations[].state` | 区域存储状态：`synced` (已同步)、`syncing` (同步中)、`stale` (过期) |
| `locations[].last_sync_at` | 该区域最后同步时间 |
| `primary` | 主副本所在区域 |
| `replicas` | 副本总数 |

**错误响应**

```
Status: 404 Not Found
```

```json
{
  "error": "not found"
}
```

---

## 区域管理

### GET /regions

获取所有活跃区域列表（排除 offline 状态的区域）。

**响应**

```
Status: 200 OK
```

```json
[
  {
    "id": "region-beijing",
    "name": "Beijing Region",
    "endpoint": "http://region-beijing:8080",
    "location": {
      "latitude": 39.9042,
      "longitude": 116.4074,
      "city": "Beijing",
      "country": "China"
    },
    "capacity": {
      "total_bytes": 107374182400,
      "used_bytes": 21474836480,
      "free_bytes": 85899345920
    },
    "status": {
      "state": "healthy",
      "sync_lag": 0,
      "load_level": 0.35,
      "last_check_at": "2025-01-15T11:00:00Z"
    },
    "joined_at": "2025-01-10T08:00:00Z",
    "last_seen_at": "2025-01-15T11:00:00Z"
  }
]
```

**区域状态说明**

| 状态 | 说明 | 触发条件 |
|------|------|----------|
| `healthy` | 健康 | 正常心跳 |
| `degraded` | 降级 | 超过 1 分钟未收到心跳 |
| `offline` | 离线 | 超过 5 分钟未收到心跳 |

---

### GET /regions/:id

获取指定区域的详细信息。

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | string (path) | 区域 ID |

**响应**

```
Status: 200 OK
```

响应结构同 `GET /regions` 中的单个元素。

**错误响应**

```
Status: 404 Not Found
```

```json
{
  "error": "not found"
}
```

---

### POST /regions/:id/heartbeat

接收区域心跳上报，更新区域健康状态。

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | string (path) | 区域 ID |

**请求体**

```json
{
  "state": "healthy",
  "sync_lag": 0,
  "load_level": 0.35,
  "last_check_at": "2025-01-15T11:00:00Z"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `state` | string | 区域自报状态：`healthy` / `degraded` |
| `sync_lag` | int64 | 同步延迟（待同步的变更数量） |
| `load_level` | float64 | 负载水平 (0.0 - 1.0) |
| `last_check_at` | string (ISO 8601) | 自检时间 |

**响应**

```
Status: 204 No Content
```

**错误响应**

```
Status: 400 Bad Request
```

```json
{
  "error": "invalid request body"
}
```

```
Status: 404 Not Found
```

```json
{
  "error": "not found"
}
```

**示例**

```bash
curl -X POST http://localhost:8081/api/v1/regions/region-beijing/heartbeat \
  -H "Content-Type: application/json" \
  -d '{
    "state": "healthy",
    "sync_lag": 0,
    "load_level": 0.2,
    "last_check_at": "2025-01-15T11:00:00Z"
  }'
```

---

## 同步操作

### GET /sync/pending/:region_id

获取指定区域待同步的变更事件列表。

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `region_id` | string (path) | 区域 ID |

**响应**

```
Status: 200 OK
```

```json
[
  {
    "event_id": "evt-001",
    "file_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "create",
    "source_region": "region-shanghai",
    "timestamp": "2025-01-15T10:30:00Z",
    "metadata": { ... }
  }
]
```

无待同步事件时返回空数组 `[]` 或 `null`。

---

## 错误格式

所有错误响应统一为以下格式：

```json
{
  "error": "错误描述信息"
}
```

常见 HTTP 状态码：

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 204 | 操作成功，无响应体 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |
