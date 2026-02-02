# Mock 微信支付服务

这是一个独立的服务，模拟微信支付服务端的行为，用于开发和测试。

## 功能

提供和微信支付 API v3 相同的接口：
- `POST /v3/pay/transactions/native` - Native 下单（预支付）
- `GET /v3/pay/transactions/out-trade-no/:outTradeNo` - 查询订单

额外的测试接口：
- `POST /mock/pay/:outTradeNo` - 模拟支付成功
- `GET /mock/orders` - 查看所有订单

## 启动服务

```bash
cd backend/cmd/mock-wechat
go run main.go
```

服务运行在 `http://localhost:8888`

## 使用方法

### 1. 修改 Payment 服务配置

```yaml
# config/dev.yaml
wechat:
  api_base_url: "http://localhost:8888"  # 指向 Mock 服务
  app_id: "mock_app_id"
  mch_id: "mock_mch_id"
  # 其他配置可以随意填写，Mock 服务不验证
```

### 2. 测试流程

```bash
# 1. 启动 Mock 微信支付服务
cd backend/cmd/mock-wechat
go run main.go

# 2. 启动 Payment 服务
cd backend/cmd/payment
go run main.go

# 3. 创建预支付订单
curl -X POST http://localhost:8092/payment/prepay \
  -d '{"amount": 100, "order_no": "ORDER001"}'

# 4. 模拟支付成功
curl -X POST http://localhost:8888/mock/pay/ORDER001

# 5. 查询支付状态
curl http://localhost:8092/payment/query/ORDER001
```

## 架构说明

```
Payment Service (不需要修改)
    ↓ 调用微信支付 API
    ↓
开发环境: http://localhost:8888 (Mock Wechat Server)
生产环境: https://api.mch.weixin.qq.com (真实微信)
```

Payment 服务的代码完全不需要改动，只需要配置不同的 `api_base_url` 即可。
