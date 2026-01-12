# 区域层 (Region Layer) 设计

## 1. 概述

区域层是 JzSE 分布式文件服务系统的"四肢"，部署在每个地理区域，直接服务本地用户。设计目标是提供 **低延迟、高吞吐** 的文件操作体验，同时支持 **区域自治**。

## 2. 核心组件架构

```
┌──────────────────────────────────────────────────────────────┐
│                      Region Service                          │
├──────────────────────────────────────────────────────────────┤
│  API Layer: HTTP/REST | gRPC | WebSocket                     │
├──────────────────────────────────────────────────────────────┤
│  Business Logic: FileHandler | ACLHandler | VersionCtrl     │
├──────────────────────────────────────────────────────────────┤
│  Data Layer: LocalMetadata(BadgerDB) | LocalStorage(FS)     │
├──────────────────────────────────────────────────────────────┤
│  Sync Layer: SyncAgent | ChangeQueue | ConflictDetector     │
└──────────────────────────────────────────────────────────────┘
```

## 3. REST API 设计

```go
// 文件操作
POST   /api/v1/files                    // 上传文件
GET    /api/v1/files/:id                // 下载文件
DELETE /api/v1/files/:id                // 删除文件
PUT    /api/v1/files/:id                // 更新文件

// 目录操作
POST   /api/v1/directories              // 创建目录
GET    /api/v1/directories/:path        // 列出目录
DELETE /api/v1/directories/:path        // 删除目录

// 版本与管理
GET    /api/v1/files/:id/versions       // 版本历史
GET    /api/v1/health                   // 健康检查
```

## 4. 核心数据模型

```go
type FileMetadata struct {
    ID            string            `json:"id"`
    Name          string            `json:"name"`
    Path          string            `json:"path"`
    Size          int64             `json:"size"`
    ContentHash   string            `json:"content_hash"`
    Version       int64             `json:"version"`
    VectorClock   map[string]uint64 `json:"vector_clock"`
    OriginRegion  string            `json:"origin_region"`
    LocalState    LocalState        `json:"local_state"`
    SyncState     SyncState         `json:"sync_state"`
    CreatedAt     time.Time         `json:"created_at"`
    UpdatedAt     time.Time         `json:"updated_at"`
}

type LocalState string // present | pending | deleted
type SyncState string  // synced | pending | conflict
```

## 5. 存储后端接口

```go
type StorageBackend interface {
    Put(ctx context.Context, key string, reader io.Reader, size int64) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

支持: 本地文件系统、MinIO、S3

## 6. 同步机制

- **Push Mode**: 变更立即推送
- **Batch Mode**: 累积后批量推送
- **Pull Mode**: 定期拉取变更

## 7. 区域自治

断网时继续服务本地请求，变更累积在队列，恢复后自动重放。

详见 [同步机制设计](./sync-mechanism.md)
