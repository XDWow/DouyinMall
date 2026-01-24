# 订单微服务 - 批量取消订单和Outbox模式实现

## 概述

本实现包含了订单微服务中批量取消过期订单和Outbox模式的完整解决方案，遵循DDD和清晰架构原则。

## 架构设计

### 1. 批量取消订单流程

**问题**：原来直接批量修改数据库不行，因为需要走完整的业务逻辑（状态变更 + 发送消息 + outbox记录）

**解决方案**：
- 创建 `BatchCancelOrderUseCase` 专门处理批量取消
- 在事务中批量更新订单状态 + 批量写入outbox
- 异步批量发送MQ消息（快路径）
- Outbox worker定期扫描（慢路径兜底）

### 2. 实现的核心组件

#### Domain层 (领域层)
- `Order` - 订单聚合根
- `OrderRepository` - 订单仓储接口
  - `FindExpiredOrders()` - 查找过期订单
  - `BatchUpdateStatus()` - 批量更新状态
- `OutboxRepository` - Outbox仓储接口
  - `BatchAdd()` - 批量添加outbox事件
  - `BatchMarkSent()` - 批量标记已发送
  - `ListPending()` - 查询待发送事件
- `OutboxEvent` - Outbox事件（包含ID和业务事件）

#### UseCase层 (应用层)
- `BatchCancelOrderUseCase` - 批量取消订单用例
  - 批量更新订单状态（Pending → Canceled）
  - 批量写入outbox（慢路径兜底）
  - 批量发送MQ消息（快路径）
  - 失败处理和重试计数

#### Infrastructure层 (基础设施层)
- `orderRepository` - 订单仓储实现
  - `FindExpiredOrders()` - 查询30分钟内未支付的订单
  - `BatchUpdateStatus()` - 使用IN语句批量更新
- `outboxRepository` - Outbox仓储实现
  - `BatchAdd()` - 使用`CreateInBatches()`批量插入
  - `BatchMarkSent()` - 批量标记已发送
  - `ListPending()` - 返回包含ID的OutboxEvent

#### Job层 (定时任务层)
1. **CheckExpiredJob** - 扫描过期订单
   - 每分钟执行一次
   - 查找超过30分钟未支付的订单
   - 调用BatchCancelOrderUseCase批量取消
   
2. **OutboxWorkerJob** - 扫描Outbox事件
   - 定期执行（如每30秒）
   - 分页查询pending的outbox事件
   - 批量发送消息到MQ
   - 批量标记发送成功的事件
   - 对失败的事件增加重试次数

## 关键设计决策

### 1. 为什么需要批量操作？

单个取消订单的问题：
```go
// 不推荐：单个处理，效率低
for _, order := range expiredOrders {
    uc.CancelOrder(order.ID)  // N次数据库事务 + N次MQ发送
}
```

批量处理的优势：
```go
// 推荐：批量处理，高效且一致
uc.BatchCancelOrder(expiredOrders)  // 1次数据库事务 + 批量MQ发送
批量MQ发送：
业务事件不能批量，是因为事件的最小语义单位是一次业务事实。
把多个订单合并成一条消息，会导致失败粒度失控、幂等复杂、重试放大。
真正的性能优化应该在 MQ 发送层，通过客户端 batching 合并网络 IO，而不是在业务层合并事件。
```

- **性能优化**：减少数据库往返次数，使用批量SQL（IN语句）
- **事务一致性**：所有订单状态和outbox事件在同一个事务中提交
- **资源利用**：减少连接开销，提高吞吐量

### 2. MQ批量发送的设计哲学

**核心原则**："失败应该被隔离，而不是被放大"

**错误做法**（业务层合并）：
```go
// ❌ 把多个订单合并为一个消息
message := BatchCancelEvent{OrderIDs: [1,2,3,4,5]}
producer.Send(message)  // 一个失败，全部失败
```

**正确做法**（发送层批量）：
```go
// ✅ 每个订单一个独立消息，批量发送
messages := []Event{
    {OrderID: 1}, {OrderID: 2}, {OrderID: 3}
}
producer.SendMessages(messages)  // 可以知道具体哪个失败了
```

**优势**：
- **失败隔离**：单个订单失败不影响其他订单
- **精确重试**：只重试失败的消息
- **性能优化**：批量发送减少网络往返
- **消息独立**：消费端处理简单，不需要拆分

### 3. Outbox模式的双路径设计

**快路径（Fast Path）**：
- 事务提交后立即异步批量发送MQ消息
- 成功后批量标记outbox为已发送
- 大多数情况下消息会立即送达

**慢路径（Slow Path）**：
- OutboxWorkerJob定期扫描pending的事件
- 批量发送失败的消息（保持消息独立性）
- 提供最终一致性保证

### 4. 事务边界

### 4. 事务边界

```go
// 在事务内完成
err := uc.tx.WithTx(ctx, func(ctx context.Context) error {
    // 1. 批量更新订单状态
    uc.orderRepo.BatchUpdateStatus(...)
    // 2. 批量写入outbox（每个订单一条记录）
    uc.outboxRepo.BatchAdd(...)
    return nil
})

// 事务外批量发送（快路径）
go func() {
    // 使用MQ的批量发送API，但每个订单仍是独立消息
    errs := producer.SendMessages(events)
    // 精确知道哪些成功、哪些失败
    for i, err := range errs {
        if err != nil {
            // 只重试失败的
        }
    }
}()
```

这样确保：
- 订单状态变更和outbox记录是原子的
- MQ批量发送不阻塞事务提交
- 消息保持独立，失败被隔离
- 即使MQ发送失败，也有outbox兜底

## 使用示例

### 注册Job到调度器

```go
// 在IOC容器中注册
func InitJobs(
    orderRepo domain.OrderRepository,
    outboxRepo domain.OutboxRepository,
    batchCancelUC *usecase.BatchCancelOrderUseCase,
    producer *mq.SaramaProducer,
    log logger.LoggerV1,
) []job.Job {
    return []job.Job{
        // 每分钟扫描一次过期订单
        job.NewCheckExpiredJob(orderRepo, batchCancelUC, log),
        // 每30秒扫描一次pending的outbox事件
        job.NewOutboxWorkerJob(outboxRepo, producer, log),
    }
}
```

### 配置定时任务

```go
// 使用cron或者其他调度器
scheduler.AddJob("@every 1m", checkExpiredJob)   // 每分钟
scheduler.AddJob("@every 30s", outboxWorkerJob)  // 每30秒
```

## 数据库索引建议

```sql
-- 订单表索引
CREATE INDEX idx_order_status_expired ON orders(status, expired_at) 
WHERE status = 1;  -- 只索引Pending状态

-- Outbox表索引
CREATE INDEX idx_outbox_status ON outbox_events(status) 
WHERE status = 1;  -- 只索引Pending状态

-- 组合索引用于重试控制
CREATE INDEX idx_outbox_status_retry ON outbox_events(status, retry_count);
```

## 监控指标

建议监控以下指标：
1. **过期订单数量** - 每次扫描发现的过期订单数
2. **批量取消成功率** - 成功取消的订单比例
3. **Outbox积压数量** - pending状态的事件数量
4. **Outbox发送成功率** - 消息发送成功的比例
5. **重试次数分布** - 各重试次数的事件分布

## 故障恢复

### 场景1：MQ服务宕机
- 快路径发送失败，增加重试次数
- OutboxWorkerJob继续尝试发送
- MQ恢复后，积压的消息会被逐步发送

### 场景2：数据库主从切换
- 事务提交失败，整个批量取消会回滚
- 下次定时任务继续处理这些订单

### 场景3：应用重启
- 未完成的outbox事件保留在数据库
- 重启后OutboxWorkerJob继续处理

## 性能考虑

- **批量大小**：默认100，可根据实际情况调整
- **超时时间**：30秒，确保长时间运行的job不会永久阻塞
- **并发控制**：Job通过调度器串行执行，避免冲突
- **数据库连接池**：确保有足够的连接支持批量操作

## 最佳实践

1. ✅ **分批处理**：大量订单时避免一次性处理所有
2. ✅ **幂等性**：支持重复执行不会产生副作用
3. ✅ **可观测性**：充分的日志记录和指标监控
4. ✅ **优雅降级**：单个失败不影响整批处理
5. ✅ **事务边界清晰**：DB操作在事务内，MQ发送在事务外

## 文件清单

- `domain/repository.go` - 扩展接口定义
- `domain/event.go` - 添加OutboxEvent结构
- `usecase/batch_cancel_order.go` - 批量取消用例（新增）
- `infra/repository/order.go` - 订单仓储实现
- `infra/repository/outbox.go` - Outbox仓储实现
- `job/checkExpired_job.go` - 过期订单扫描Job（重构）
- `job/outbox_worker.go` - Outbox事件扫描Job（重构）
