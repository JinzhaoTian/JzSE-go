# Region Service API 文档

**Base URL**: `http://localhost:8080/api/v1`

---

## 健康检查

### GET /health

检查 Region 服务健康状态。

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

## 文件操作

### POST /files

上传文件。

**请求**

Content-Type: `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | file | 是 | 上传的文件内容 |
| `path` | string | 否 | 目标路径，默认 `/` |

**响应**

```
Status: 201 Created
```

```json
{
  "FileID": "550e8400-e29b-41d4-a716-446655440000",
  "Path": "/docs/test.txt",
  "Size": 1024,
  "ContentHash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "Version": 1,
  "CreatedAt": "2025-01-15T10:30:00Z"
}
```

**错误响应**

```
Status: 400 Bad Request
```

```json
{
  "error": "no file provided"
}
```

**示例**

```bash
curl -X POST http://localhost:8080/api/v1/files \
  -F "file=@document.pdf" \
  -F "path=/docs"
```

---

### GET /files/:id

下载文件。

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | string (path) | 文件 ID (UUID) |

**响应**

```
Status: 200 OK
Content-Type: <文件原始 MIME 类型>
Content-Disposition: attachment; filename=<文件名>
X-File-ID: <文件 ID>
X-Content-Hash: <SHA-256 哈希>
```

响应体为文件二进制内容。

**错误响应**

```
Status: 404 Not Found
```

```json
{
  "error": "file not found"
}
```

**示例**

```bash
curl http://localhost:8080/api/v1/files/550e8400-e29b-41d4-a716-446655440000 \
  -o downloaded_file.pdf
```

---

### DELETE /files/:id

删除文件。文件会被标记为已删除（tombstone），并等待同步到其他区域。

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | string (path) | 文件 ID (UUID) |

**响应**

```
Status: 204 No Content
```

**错误响应**

```
Status: 404 Not Found
```

```json
{
  "error": "file not found"
}
```

**示例**

```bash
curl -X DELETE http://localhost:8080/api/v1/files/550e8400-e29b-41d4-a716-446655440000
```

---

### GET /files/:id/metadata

获取文件元数据。

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
  "version": 1,
  "vector_clock": {
    "region-beijing": 1
  },
  "owner_id": "anonymous",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z",
  "created_by": "anonymous",
  "updated_by": "anonymous",
  "origin_region": "region-beijing",
  "local_state": "present",
  "sync_state": "pending"
}
```

**字段说明**

| 字段 | 说明 |
|------|------|
| `local_state` | 本地存储状态：`present` (存在)、`pending` (等待下载)、`deleted` (已删除) |
| `sync_state` | 同步状态：`synced` (已同步)、`pending` (待同步)、`conflict` (冲突) |
| `vector_clock` | 向量时钟，记录各区域的修改序号，用于冲突检测 |
| `origin_region` | 文件创建的源区域 |

**错误响应**

```
Status: 404 Not Found
```

```json
{
  "error": "file not found"
}
```

**示例**

```bash
curl http://localhost:8080/api/v1/files/550e8400-e29b-41d4-a716-446655440000/metadata
```

---

## 目录操作

### GET /directories/*path

列出指定目录下的文件和子目录。

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `path` | string (path) | 目录路径，例如 `/docs` |

**响应**

```
Status: 200 OK
```

```json
{
  "path": "/docs",
  "entries": [
    {
      "name": "test.txt",
      "path": "/docs/test.txt",
      "is_dir": false,
      "size": 1024,
      "updated_at": "2025-01-15T10:30:00Z"
    },
    {
      "name": "images",
      "path": "/docs/images",
      "is_dir": true,
      "updated_at": "2025-01-15T09:00:00Z"
    }
  ]
}
```

**示例**

```bash
# 列出根目录
curl http://localhost:8080/api/v1/directories/

# 列出子目录
curl http://localhost:8080/api/v1/directories/docs
```

---

## 区域状态

### GET /region/status

获取当前区域的运行状态。

**响应**

```
Status: 200 OK
```

```json
{
  "status": "healthy",
  "sync_state": "connected"
}
```

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
| 201 | 创建成功（文件上传） |
| 204 | 操作成功，无响应体（删除） |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |
