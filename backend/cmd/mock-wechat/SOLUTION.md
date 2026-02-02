# Mock 微信支付解决方案

## 问题

开发支付微服务时，没有真实的微信支付商户ID，无法测试支付功能。

## 解决方案

**创建独立的 Mock 微信支付服务端**，模拟微信支付服务器的行为。

## 架构

```
┌─────────────────────────────────────┐
│   Payment Service (你的服务)        │
│   - 业务逻辑不变                     │
│   - 只需配置不同的 API 地址          │
└──────────────┬──────────────────────┘
               │ HTTP 调用
               ↓
       ┌──────────────┐
       │  环境切换     │
       └──────┬───────┘
              │
      ┌───────┴────────┐
      ↓                ↓
┌──────────┐    ┌─────────────────┐
│ 开发环境  │    │   生产环境       │
│          │    │                 │
│ Mock服务 │    │ 真实微信支付     │
│ :8888    │    │ api.mch.weixin  │
└──────────┘    └─────────────────┘
```

## 使用步骤

### 1. 启动 Mock 微信支付服务

```bash
cd backend/cmd/mock-wechat
go run main.go
```

服务运行在 `http://localhost:8888`，提供以下接口：

- `POST /v3/pay/transactions/native` - 预支付（和微信 API 相同）
- `GET /v3/pay/transactions/out-trade-no/:outTradeNo` - 查询订单
- `POST /mock/pay/:outTradeNo` - 模拟支付成功（测试专用）
- `GET /mock/orders` - 查看所有订单（测试专用）

### 2. 配置 Payment 服务

**开发环境** (`config/dev.yaml`):
```yaml
payment:
  mode: "mock"

wechat:
  api_base_url: "http://localhost:8888"  # 指向 Mock 服务
  app_id: "mock_app_id"
  mch_id: "mock_mch_id"
  # 其他配置可以随意填写
```

**生产环境** (`config/prod.yaml`):
```yaml
payment:
  mode: "real"

wechat:
  api_base_url: "https://api.mch.weixin.qq.com"  # 真实微信
  app_id: "真实AppID"
  mch_id: "真实商户号"
  cert_serial_no: "真实证书序列号"
  private_key_path: "/path/to/real/key.pem"
  api_v3_key: "真实APIv3密钥"
```

### 3. 测试流程

```bash
# Terminal 1: 启动 Mock 微信服务
cd backend/cmd/mock-wechat
go run main.go

# Terminal 2: 启动 Payment 服务
cd backend/cmd/payment
go run main.go

# Terminal 3: 测试
# 创建预支付
grpcurl -plaintext -d '{
  "amt": {"total": 100, "currency": "CNY"},
  "biz_trade_no": "ORDER_20260128_001",
  "description": "测试商品"
}' localhost:8092 payment.PaymentService/NativePrepay

# 模拟支付成功
curl -X POST http://localhost:8888/mock/pay/ORDER_20260128_001

# 查询支付状态
grpcurl -plaintext -d '{
  "biz_trade_no": "ORDER_20260128_001"
}' localhost:8092 payment.PaymentService/GetPayment
```

## 关键点

### ✅ 正确的做法

1. **Mock 是独立服务**：在 `cmd/mock-wechat/` 中，不影响 Payment 代码
2. **Payment 代码不变**：只依赖配置切换，业务逻辑完全不动
3. **模拟服务端行为**：Mock 服务模拟微信支付服务器，不是客户端

### ❌ 错误的做法

1. ~~在 UseCase 中写 Mock 逻辑~~ - 业务层不应该有 Mock 代码
2. ~~在 Infra 中写 Mock 实现~~ - 基础设施层应该是真实实现
3. ~~通过接口抽象切换~~ - 不需要，直接切换 API 地址即可

## 优势

✅ **完全隔离**：Mock 服务独立运行，不污染 Payment 代码  
✅ **真实模拟**：模拟微信服务端，测试更接近生产  
✅ **配置切换**：一行配置切换开发/生产环境  
✅ **易于调试**：可以查看所有 Mock 订单，手动触发支付成功  
✅ **团队共享**：Mock 服务可以部署到测试环境，整个团队使用  

## 目录结构

```
backend/
├── cmd/
│   ├── payment/              # 支付服务（业务代码）
│   └── mock-wechat/          # Mock 微信支付服务（独立）
│       ├── main.go
│       └── README.md
│
└── internal/payment/
    ├── usecase/              # 业务逻辑（不变）
    ├── domain/               # 领域模型（不变）
    ├── ioc/
    │   └── wechat.go         # 根据 mode 配置跳过证书验证
    └── config/
        └── dev.yaml          # 配置 api_base_url
```

## 扩展

未来可以增强 Mock 服务：
- 模拟支付失败场景
- 模拟网络超时
- 支持退款接口
- 记录所有请求日志
- 提供 Web UI 管理界面

---

这才是正确的 Mock 方式！🎉
