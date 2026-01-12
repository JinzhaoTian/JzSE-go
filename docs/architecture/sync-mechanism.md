# 数据同步机制设计

## 1. 概述

JzSE 采用 **最终一致性** 模型，通过异步同步实现跨区域数据一致性。核心设计原则：

- 本地操作优先，不阻塞用户
- 异步传播变更到其他区域
- 使用向量时钟检测冲突
- 支持多种冲突解决策略

## 2. 向量时钟 (Vector Clock)

用于追踪分布式系统中的因果关系：

```go
type VectorClock map[string]uint64  // region_id -> logical_clock

func (vc VectorClock) Increment(regionID string) {
    vc[regionID]++
}

func (vc VectorClock) Merge(other VectorClock) {
    for k, v := range other {
        if vc[k] < v {
            vc[k] = v
        }
    }
}

func (vc VectorClock) Compare(other VectorClock) Relation {
    // BEFORE: vc 发生在 other 之前
    // AFTER: vc 发生在 other 之后
    // EQUAL: 相同
    // CONCURRENT: 并发（可能冲突）
}
```

## 3. 变更事件流

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  Region A   │    │ Coordinator │    │  Region B   │
└──────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                  │
       │ 1. Write File    │                  │
       │──────────────────>                  │
       │                  │                  │
       │ 2. ACK + Update  │                  │
       │ Global Metadata  │                  │
       │<──────────────────                  │
       │                  │                  │
       │                  │ 3. Broadcast     │
       │                  │ ChangeEvent      │
       │                  │──────────────────>
       │                  │                  │
       │                  │ 4. ACK Sync      │
       │                  │<──────────────────
       │                  │                  │
```

## 4. 同步队列设计

```go
type SyncQueue struct {
    pending   *list.List      // 待发送队列
    inflight  map[string]bool // 发送中
    retryHeap *heap.Heap      // 重试堆
    persister Persister       // 持久化
}

type SyncItem struct {
    ID        string
    Event     *ChangeEvent
    Priority  int
    Attempts  int
    NextRetry time.Time
}
```

### 优先级策略

| 优先级 | 事件类型 | 描述 |
|--------|----------|------|
| 0 (最高) | DELETE | 删除需要快速传播 |
| 1 | CREATE | 新文件创建 |
| 2 | UPDATE | 文件更新 |
| 3 | METADATA | 仅元数据变更 |

## 5. 冲突检测与解决

### 5.1 冲突场景

```
Timeline:
         Region A          Region B
            │                  │
   T1       │ Write v1         │
            │                  │
   T2       │                  │ Write v1'
            │                  │
   T3       │──── Sync ────────│
            │      ⚡           │
            │   CONFLICT!      │
```

### 5.2 解决策略

**Last-Writer-Wins (LWW)**
```go
func ResolveLWW(a, b *FileMetadata) *FileMetadata {
    if a.UpdatedAt.After(b.UpdatedAt) {
        return a
    }
    return b
}
```

**Fork (分叉)**
```go
func ResolveFork(a, b *FileMetadata) []*FileMetadata {
    a.Path = a.Path + ".conflict-a"
    b.Path = b.Path + ".conflict-b"
    return []*FileMetadata{a, b}
}
```

## 6. 断网恢复

```go
func (s *SyncAgent) Reconnect(ctx context.Context) error {
    // 1. 获取离线期间的远程变更
    remoteChanges, _ := s.coordinator.GetChangesSince(s.lastSyncTime)
    
    // 2. 获取本地累积的变更
    localChanges := s.queue.GetAll()
    
    // 3. 检测冲突
    conflicts := DetectConflicts(localChanges, remoteChanges)
    
    // 4. 解决冲突
    for _, c := range conflicts {
        resolved := s.resolver.Resolve(c, s.config.ConflictStrategy)
        s.applyResolution(resolved)
    }
    
    // 5. 应用无冲突的远程变更
    s.applyRemoteChanges(remoteChanges)
    
    // 6. 推送本地变更
    s.pushLocalChanges(localChanges)
}
```

## 7. 监控指标

```
sync_events_total{region, type}     // 同步事件计数
sync_queue_size{region}             // 队列大小
sync_lag_seconds{region}            // 同步延迟
conflicts_total{region, strategy}   // 冲突计数
sync_failures_total{region}         // 同步失败计数
```
