# 全局协调层 (Global Coordination Layer) 设计

## 1. 概述

全局协调层是 JzSE 系统的"大脑"，部署在核心数据中心，负责：
- 维护权威的全局元数据视图
- 协调跨区域数据同步
- 检测和解决数据冲突
- 管理区域注册和健康状态

## 2. 核心组件架构

```
┌─────────────────────────────────────────────────────────────┐
│                  Global Coordinator                         │
├─────────────────────────────────────────────────────────────┤
│  API Gateway: gRPC Server | Admin REST API                  │
├─────────────────────────────────────────────────────────────┤
│  Core Services:                                              │
│  ┌────────────┐ ┌────────────┐ ┌────────────────┐          │
│  │  Metadata  │ │   Sync     │ │   Conflict     │          │
│  │  Manager   │ │   Engine   │ │   Resolver     │          │
│  └────────────┘ └────────────┘ └────────────────┘          │
│  ┌────────────┐ ┌────────────┐                              │
│  │  Region    │ │   Event    │                              │
│  │  Registry  │ │    Bus     │                              │
│  └────────────┘ └────────────┘                              │
├─────────────────────────────────────────────────────────────┤
│  Storage: etcd (Metadata) | PostgreSQL (Users/ACL)         │
└─────────────────────────────────────────────────────────────┘
```

## 3. Global Metadata Manager

维护全局文件元数据的权威视图：

```go
type GlobalMetadataManager interface {
    // 获取文件元数据
    Get(ctx context.Context, fileID string) (*GlobalFileMetadata, error)
    
    // 更新元数据 (带向量时钟比较)
    Update(ctx context.Context, meta *GlobalFileMetadata) error
    
    // 注册新文件
    Register(ctx context.Context, meta *GlobalFileMetadata) error
    
    // 查询文件位置
    GetLocations(ctx context.Context, fileID string) ([]RegionLocation, error)
    
    // 批量查询
    BatchGet(ctx context.Context, fileIDs []string) ([]*GlobalFileMetadata, error)
}

type GlobalFileMetadata struct {
    FileMetadata                         // 嵌入基础元数据
    Locations    []RegionLocation        // 文件所在区域列表
    Primary      string                  // 主区域
    Replicas     int                     // 副本数
}

type RegionLocation struct {
    RegionID    string
    State       string    // synced | syncing | stale
    LastSyncAt  time.Time
}
```

## 4. Sync Engine

协调跨区域数据同步：

```go
type SyncEngine interface {
    // 处理来自区域的变更事件
    HandleChange(ctx context.Context, event *ChangeEvent) error
    
    // 广播变更到其他区域
    BroadcastChange(ctx context.Context, event *ChangeEvent, excludeRegion string) error
    
    // 获取某区域的待同步变更
    GetPendingChanges(ctx context.Context, regionID string, since time.Time) ([]*ChangeEvent, error)
    
    // 确认同步完成
    AckSync(ctx context.Context, regionID string, eventIDs []string) error
}
```

### 同步策略

| 策略 | 描述 | 适用场景 |
|------|------|----------|
| **Eager** | 变更立即广播到所有区域 | 对一致性要求高的关键文件 |
| **Lazy** | 变更延迟批量广播 | 一般文件，降低网络开销 |
| **On-Demand** | 仅在访问时同步 | 冷数据，节省存储 |

## 5. Conflict Resolver

处理并发写入冲突：

```go
type ConflictResolver interface {
    // 检测冲突
    Detect(ctx context.Context, local, remote *FileMetadata) (*Conflict, error)
    
    // 解决冲突
    Resolve(ctx context.Context, conflict *Conflict, strategy ResolutionStrategy) (*FileMetadata, error)
}

type Conflict struct {
    FileID       string
    LocalVersion *FileMetadata
    RemoteVersion *FileMetadata
    DetectedAt   time.Time
}

type ResolutionStrategy string
const (
    StrategyLWW      ResolutionStrategy = "last_writer_wins"  // 最后写入获胜
    StrategyMerge    ResolutionStrategy = "merge"              // 尝试合并
    StrategyManual   ResolutionStrategy = "manual"             // 人工解决
    StrategyFork     ResolutionStrategy = "fork"               // 分叉保留两个版本
)
```

### 向量时钟比较

```go
func CompareVectorClocks(vc1, vc2 map[string]uint64) ClockRelation {
    // 返回: BEFORE, AFTER, EQUAL, CONCURRENT
}
```

## 6. Region Registry

管理区域注册和健康状态：

```go
type RegionRegistry interface {
    // 注册区域
    Register(ctx context.Context, region *RegionInfo) error
    
    // 注销区域
    Deregister(ctx context.Context, regionID string) error
    
    // 心跳上报
    Heartbeat(ctx context.Context, regionID string, status *RegionStatus) error
    
    // 获取所有活跃区域
    GetActiveRegions(ctx context.Context) ([]*RegionInfo, error)
    
    // 获取区域详情
    GetRegion(ctx context.Context, regionID string) (*RegionInfo, error)
}

type RegionInfo struct {
    ID          string
    Name        string
    Endpoint    string           // gRPC 地址
    Location    GeoLocation      // 地理位置
    Capacity    StorageCapacity
    Status      RegionStatus
    JoinedAt    time.Time
    LastSeenAt  time.Time
}

type RegionStatus struct {
    State       string  // healthy | degraded | offline
    SyncLag     int64   // 同步延迟 (事件数)
    StorageUsed int64
    LoadLevel   float64 // 0-1
}
```

## 7. Event Bus

区域间事件通知（基于 NATS）：

```go
// 事件主题设计
jzse.changes.<region_id>       // 区域变更事件
jzse.sync.requests             // 同步请求
jzse.sync.acks                 // 同步确认
jzse.regions.status            // 区域状态更新
jzse.conflicts                 // 冲突通知
```

## 8. 配置示例

```yaml
coordinator:
  id: "coordinator-main"
  
  # etcd 配置
  etcd:
    endpoints:
      - "etcd-1:2379"
      - "etcd-2:2379"
      - "etcd-3:2379"
    dial_timeout: "5s"
    
  # NATS 配置
  nats:
    url: "nats://nats-cluster:4222"
    cluster_id: "jzse-cluster"
    
  # 同步配置
  sync:
    default_strategy: "lazy"
    batch_size: 100
    broadcast_timeout: "10s"
    
  # 冲突解决
  conflict:
    default_strategy: "last_writer_wins"
    auto_resolve: true
```

## 9. 高可用设计

- 多实例部署，基于 etcd 选主
- 无状态设计，任意实例可处理请求
- 事件持久化，支持故障恢复
