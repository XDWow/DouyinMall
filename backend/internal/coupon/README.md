# 优惠券微服务 (Coupon Service)

## 功能概述

优惠券微服务提供优惠券的全生命周期管理，包括发放、锁定、核销、退还等操作。

### 核心功能

1. **优惠券发放** - 支持幂等发放，避免重复领取
2. **订单结算** - 评估订单可用券
3. **预扣锁定** - 创建订单时锁定优惠券（Unused → Locked）
4. **确认核销** - 支付成功后确认使用（Locked → Used）
5. **释放回退** - 订单取消时释放（Locked → Unused）
6. **退款退还** - 订单退款时退还（Used → Unused）
7. **过期扫描** - 定时任务标记过期券

## 架构设计

### 目录结构

```
internal/coupon/
├── domain/              # 领域层
│   ├── coupon.go       # 优惠券实体、模板、业务规则
│   ├── repository.go   # Repository接口定义
│   └── error.go        # 错误定义
├── usecase/            # 应用层
│   ├── issue_coupon.go           # 发放优惠券
│   ├── list_user_coupons.go      # 查询用户券
│   ├── evaluate_order_coupons.go # 评估订单可用券
│   ├── reserve_coupon.go         # 预扣锁定
│   ├── commit_coupon.go          # 确认核销
│   ├── release_coupon.go         # 释放
│   └── refund_coupon.go          # 退还
├── infra/              # 基础设施层
│   ├── db/             # 数据库模型
│   ├── repository/     # Repository实现
│   │   ├── coupon_repo.go     # 用户优惠券
│   │   ├── template_repo.go   # 优惠券模板
│   │   └── operation_repo.go  # 幂等操作记录
│   └── mq/             # 消息队列
│       └── consumer.go # 订单状态变更消费者
├── transport/          # 传输层
│   └── grpc/
│       └── handler.go  # gRPC Handler
├── job/                # 定时任务
│   └── expire_coupon_job.go  # 过期券扫描
├── ioc/                # 依赖注入
│   ├── db.go
│   ├── kafka.go
│   ├── grpc.go
│   └── log.go
└── config/             # 配置
    ├── config.go
    └── dev.yaml

cmd/coupon/
├── main.go             # 服务入口
└── wire.go             # Wire依赖注入配置
```

### 技术栈

- **框架**: Kitex (gRPC)
- **数据库**: MySQL + GORM
- **消息队列**: Kafka (Sarama)
- **服务注册**: Etcd
- **依赖注入**: Wire
- **日志**: Zap

## 核心设计

### 1. 状态机设计

优惠券状态流转：

```
Unused (未使用)
  ↓ Reserve
Locked (已锁定)
  ↓ Commit           ↓ Release
Used (已使用)      Unused (未使用)
  ↓ Refund
Unused (未使用)
```

**幂等保证**：
- 状态转换使用条件更新（WHERE status = oldStatus）
- 重复调用自动忽略（状态不匹配则不更新）

### 2. 幂等设计

**发券幂等**：
- 使用 `coupon_operations` 表记录操作
- `operation_id` 唯一索引保证幂等
- 格式：`coupon:issue:user_123_template_456_20260204`

**核销幂等**：
- 基于 `order_id` + `status` 条件更新
- 状态机保证操作幂等性

### 3. 优惠券适用性判断

在Domain层实现，符合DDD设计：

```go
// 计算订单适用金额
func (t *CouponTemplate) CalculateApplicableAmount(items []OrderItem) int64

// 检查是否达到门槛
func (t *CouponTemplate) IsApplicableToOrder(items []OrderItem) (bool, string)
```

支持：
- 全场券（所有商品）
- 商家券（指定商家）
- 品类券（指定品类）
- 商品券（指定商品）

### 4. 消息消费

**订单状态变更消费者** (`infra/mq/consumer.go`):
- Topic: `order_status_update`
- Consumer Group: `coupon-consumer`
- 处理逻辑：
  - `OrderStatusPaid` → 确认使用优惠券
  - `OrderStatusCanceled` → 释放优惠券
  - `OrderStatusRefunded` → 退还优惠券

**容错机制**：
- 本地重试3次 (saramax.Handler)
- 统一ACK，不阻塞消费
- 幂等保证重试安全

### 5. 定时任务

**过期券扫描** (`job/expire_coupon_job.go`):
```sql
UPDATE coupons 
SET status = 4 
WHERE status = 1 AND valid_to < NOW()
```

- 执行频率：建议每小时
- 一次性批量更新，无LIMIT
- 异常告警：单次>5000条

## API接口

### 用户侧

1. **ListUserCoupons** - 查询用户优惠券列表
2. **ListAvailableCoupons** - 查询订单可用券（结算页）
3. **ReserveCoupon** - 预扣优惠券
4. **CommitCoupon** - 确认核销
5. **ReleaseCoupon** - 释放预扣
6. **RefundCoupon** - 退还优惠券

### 管理侧

1. **IssueCoupon** - 发放优惠券
2. **CreateCouponTemplate** - 创建优惠券模板（预留）

## 数据库设计

### 核心表

1. **coupon_templates** - 优惠券模板
2. **coupons** - 用户优惠券（券实例）
3. **coupon_operations** - 幂等操作记录

详见：`sql/schema.sql`

## 启动服务

### 1. 生成Wire代码

```bash
cd cmd/coupon
wire
```

### 2. 启动服务

```bash
go run cmd/coupon/*.go
```

### 3. 配置文件

修改 `internal/coupon/config/dev.yaml`：
- 数据库连接
- Redis地址
- Kafka地址
- Etcd地址
- gRPC端口

## 依赖服务

- MySQL (端口: 13306)
- Kafka (端口: 19092)
- Etcd (端口: 12379)

## 测试

```bash
# 单元测试
go test ./internal/coupon/...

# 集成测试（需要启动依赖服务）
go test -tags=integration ./internal/coupon/...
```

## 监控指标

- 优惠券发放量
- 优惠券使用率
- 过期优惠券数量
- API响应时间
- 消息消费延迟

## 注意事项

1. **幂等键格式**：发券时必须传入合法的 `operation_id`
2. **状态一致性**：优惠券状态变更依赖订单状态消息，需确保Kafka正常
3. **过期处理**：虽然有定时任务，但查询时仍会过滤过期时间
4. **并发控制**：预扣使用条件更新，天然防止超卖

## 后续优化

- [ ] 添加缓存层（Redis）
- [ ] 优惠券模板管理接口完善
- [ ] 多券叠加使用策略
- [ ] 自动计算最优惠券组合
- [ ] 监控和告警
- [ ] 压测和性能优化
