# Payment 服务架构设计

## 服务概览

Payment 服务负责处理支付相关的业务逻辑，包括：
- 预支付（创建支付单）
- 接收微信支付回调
- 同步超时订单状态
- 查询支付状态

## 架构分层

```
cmd/payment/              # 应用入口
├── main.go              # 主程序（启动 gRPC + HTTP + Job）
└── wire.go              # 依赖注入配置

internal/payment/
├── domain/              # 领域模型
│   ├── payment.go      # 支付实体
│   └── repository.go   # 仓储接口
│
├── usecase/            # 业务用例（核心业务逻辑）
│   ├── wechat_callback_uc.go      # 处理微信回调
│   ├── sync_wechat_order_uc.go    # 同步微信订单状态
│   ├── native_prepay_uc.go        # 预支付
│   └── get_payment_uc.go          # 查询支付
│
├── transport/          # 传输层（对外接口）
│   ├── grpc/          # gRPC 接口（其他微服务调用）
│   │   └── handler.go
│   └── http/          # HTTP 接口（微信回调）
│       └── handler.go
│
├── job/               # 定时任务
│   └── sync_wechat_order.go  # 同步超时订单
│
├── infra/             # 基础设施
│   ├── db/           # 数据库模型
│   └── repository/   # 仓储实现
│
└── ioc/              # IOC 容器
    ├── grpc.go       # gRPC 服务器
    ├── http.go       # HTTP 服务器
    ├── wechat.go     # 微信客户端
    ├── order_client.go  # 订单服务客户端
    ├── db.go         # 数据库
    ├── redis.go      # Redis
    └── log.go        # 日志
```

## 两种 Transport 职责划分

### 1. gRPC Transport（内部服务调用）
- **端口**: 8092
- **用途**: 供其他微服务调用
- **接口**:
  - `NativePrepay`: 创建预支付订单
  - `GetPayment`: 查询支付状态

### 2. HTTP Transport（外部回调）
- **端口**: 8093
- **用途**: 接收微信支付回调
- **接口**:
  - `POST /payment/wechat/callback`: 微信支付回调通知
  
**为什么分离？**
- gRPC 高性能，适合内部微服务通信
- HTTP 兼容性好，适合第三方回调
- 安全隔离：回调接口暴露公网，内部接口不暴露

## 代码复用设计

### 共用逻辑放在 UseCase 层

问题：微信回调和定时任务都需要"更新支付状态 + 同步订单状态"的逻辑

解决方案：
```go
// 1. PayCallbackUC: 处理回调数据
func (uc *PayCallbackUC) UpdatePaymentByTxn(ctx, cmd) error {
    // 核心逻辑：状态映射、更新支付、同步订单
}

// 2. SyncWechatOrderUC: 主动查询 + 复用更新逻辑
func (uc *SyncWechatOrderUC) SyncWechatInfo(ctx, bizTradeNo) error {
    result := uc.wechatClient.QueryOrder(bizTradeNo)  // 查询微信
    uc.payCallbackUC.UpdatePaymentByTxn(ctx, result)  // 复用核心逻辑 ✓
}

// 3. Job: 批量调度
func (job *SyncWechatOrderJob) Run() error {
    pmts := job.repo.FindExpiredPayment()
    for _, pmt := range pmts {
        job.syncUC.SyncWechatInfo(pmt.BizTradeNo)  // 调用 UC
    }
}
```

**优势**:
- ✅ UseCase 层复用，符合分层架构
- ✅ 职责清晰：回调处理、主动同步、定时调度各司其职
- ✅ 不破坏 domain 层纯粹性

## 启动流程

```bash
cd backend/cmd/payment
go run .
```

服务会同时启动：
1. **gRPC Server** (端口 8092): 处理内部服务调用
2. **HTTP Server** (端口 8093): 接收微信回调
3. **定时任务**: 每 30 分钟同步一次超时订单

## 配置说明

```yaml
# config/dev.yaml

# gRPC 服务配置
grpc:
  server:
    name: payment.service
    port: 8092

# HTTP 服务配置（微信回调）
http:
  server:
    port: 8093

# 微信支付配置
wechat:
  app_id: "wx1234567890"
  mch_id: "1234567890"
  cert_serial_no: "ABC123..."
  private_key_path: "/path/to/apiclient_key.pem"
  api_v3_key: "your_api_v3_key"
```

## 微信回调流程

```
微信支付平台
    ↓ POST /payment/wechat/callback
HTTP Handler (transport/http)
    ↓ 1. 验签（防伪造）
    ↓ 2. 解析请求
    ↓ 3. 调用 UC
PayCallbackUC (usecase)
    ↓ 1. 状态映射
    ↓ 2. 更新支付记录
    ↓ 3. 同步订单服务
    ↓ 4. 返回成功响应
```

## 定时任务流程

```
定时器（每 30 分钟）
    ↓
SyncWechatOrderJob
    ↓ 1. 查询超时订单
    ↓ 2. 遍历订单
SyncWechatOrderUC
    ↓ 1. 调用微信查询接口
    ↓ 2. 复用 PayCallbackUC 更新逻辑
```

## 最佳实践总结

1. **分层职责清晰**
   - Domain: 纯领域模型
   - UseCase: 业务逻辑（可复用）
   - Transport: 协议适配（gRPC/HTTP）
   - Job: 定时调度

2. **代码复用策略**
   - ✅ 共用逻辑放 UseCase，改为 public 方法
   - ❌ 不放 Domain（有外部依赖）
   - ❌ 不重复实现

3. **Transport 分离**
   - 内部调用用 gRPC（高性能）
   - 外部回调用 HTTP（兼容性）

4. **幂等性保证**
   - 微信回调可能重复发送
   - 通过数据库唯一约束 + 状态机保证幂等
