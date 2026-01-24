# 订单缓存架构

## 分层职责

```
┌─────────────────────────────────────────────┐
│           Domain (业务层)                     │
│  OrderRepository 接口（业务需求）              │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│      Infra/Repository (实现层)                │
│  ┌─────────────────────────────────────┐    │
│  │ orderRepository                     │    │
│  │ - key构造: orderKey(), userOrderListKey() │
│  │ - 缓存策略: Cache-Aside模式         │    │
│  │ - "拒绝拼好饭"原则                   │    │
│  └─────────────────────────────────────┘    │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│       Infra/Cache (存储层)                    │
│  ┌─────────────────────────────────────┐    │
│  │ OrderCache 接口                      │    │
│  │ - 纯存储操作                         │    │
│  │ - 不关心业务                         │    │
│  └─────────────────────────────────────┘    │
│  ┌─────────────────────────────────────┐    │
│  │ redisOrderCache 实现                 │    │
│  │ - Redis客户端封装                    │    │
│  └─────────────────────────────────────┘    │
└─────────────────────────────────────────────┘
```

## 缓存Key设计

```
order:{orderID}             → String(订单JSON)
order:user:{userID}:ids     → ZSet(orderID, score=createdAt)
```

**为什么不用Hash？**
1. 批量更新只有orderID，无法定位Hash的userID key
2. 单订单更新会操作整个Hash
3. 大用户会产生大Key

## 核心原则：拒绝拼好饭

```go
// ✅ 正确做法
for _, id := range orderIDs {
    order := cache.Get(id)
    if order == nil {
        break  // 任何miss，立即中断
    }
    orders = append(orders, order)
}
if len(orders) == len(orderIDs) {
    return orders  // 全命中才返回
}
// 否则fallback到DB

// ❌ 错误做法（"拼好饭"）
for _, id := range orderIDs {
    order := cache.Get(id)
    if order == nil {
        order = db.Get(id)  // 补数据
    }
    orders = append(orders, order)
}
```

**原因**：
- Redis是加速器，DB是真相
- 缓存不完整 = 和DB不一致
- 补数据会导致不同数据源混合
- 订单业务：正确性 > 性能

## 操作链路

### 1. 创建订单
```
Save(order)
  ↓
DB.Create(order)
  ↓
cache.Set(order:{orderID}, order, 30m)
  ↓
cache.ZAdd(order:user:{userID}:ids, {orderID: createdAt}, 10m)
```

### 2. 更新订单状态
```
UpdateStatus(order)
  ↓
DB.Update(order.status)
  ↓
cache.Del(order:{orderID})  // 删缓存，下次重新加载
```

### 3. 批量更新订单状态
```
BatchUpdateStatus(orderIDs, fromStatus, toStatus)
  ↓
DB.Update(WHERE id IN orderIDs AND status = fromStatus)
  ↓
cache.Del(order:1, order:2, order:3, ...)  // 直接用orderID批量删
  ↓
用户列表缓存: 不管，依赖TTL(10m)自动过期
```

**关键点**：批量更新不需要userID！

### 4. 查询用户订单列表

#### 首页（offset=0）
```
ListByUserID(userID, offset=0, limit=10)
  ↓
cache.ZRange(order:user:{userID}:ids, 0, 9, reverse=true)
  ↓ (得到orderIDs)
  ↓
for id in orderIDs:
    cache.Get(order:{id})
    if miss: break  // 拒绝拼好饭
  ↓
if 全命中:
    return orders  // ✅ 缓存命中
else:
    ↓ fallback
    DB.Query(WHERE user_id = ? ORDER BY created_at DESC)
    ↓
    回写缓存:
      cache.Set(order:{id}, order)
      cache.ZAdd(order:user:{userID}:ids, ...)
```

#### 非首页（offset>0）
```
直接查DB，不走缓存
```

## TTL设计

| Key类型 | TTL | 原因 |
|---------|-----|------|
| order:{orderID} | 30分钟 | 订单详情变化少，可以缓存久一点 |
| order:user:{userID}:ids | 10分钟 | 列表变化频繁，TTL短一些 |

## Cache-Aside模式

- **读**：先缓存，miss后读DB并回写
- **写**：先写DB，成功后删缓存（而不是更新缓存）

**为什么删而不更新？**
1. 删除成本低
2. 避免并发更新导致的数据不一致
3. 懒加载，只缓存热数据
