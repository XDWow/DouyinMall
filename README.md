# DouyinMall
字节跳动青训营抖音电商项目，项目文档：https://ocn58gfuqyel.feishu.cn/docx/PqckduAiYo98o3xpZxOcV9C0n3c

**订单状态展示**：
| 状态 | 用户看到 |
|------|----------|
| Created | 待支付 |
| Paid | 待发货 ✅ |
| Shipped | 待收货 |
| Completed | 已完成 |
| Canceled | 已取消 |
| Refunded | 已退款 |

## 后续微服务规划

```
┌─────────────────────────────────────────────────────┐
│                   Checkout Service                   │
│            (编排层，组合下单+支付+优惠券)              │
└─────────────────────────────────────────────────────┘
        │           │           │           │
        ▼           ▼           ▼           ▼
   ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
   │ Order  │ │Payment │ │Coupon  │ │Inventory│
   │Service │ │Service │ │Service │ │Service │
   └────────┘ └────────┘ └────────┘ └────────┘
                                        │
                              ┌─────────┴─────────┐
                              ▼                   ▼
                        ┌────────┐          ┌────────┐
                        │Product │          │Merchant│
                        │Service │          │Service │
                        └────────┘          └────────┘
                                                 │
                                                 ▼
                                           (发货、物流)
```

## 最终架构

| 微服务 | 职责 | 优先级 |
|--------|------|--------|
| User | 用户认证 | ✅ 已有 |
| Product | 商品管理 | ✅ 已有 |
| Cart | 购物车 | ✅ 已有 |
| Order | 订单 | ✅ 已有 |
| Payment | 支付 | ✅ 已有 |
| Inventory | 库存 | ✅ 已有 |
| Search | 搜索 | ✅ 已有 |
| **Coupon** | 优惠券 | 🔜 待开发 |
| **Merchant** | 商家+发货 | 🔜 待开发 |
| **Checkout** | 结账编排 | 🔜 待开发 |
| **AI Agent** | 智能助手 | 🔜 最后 |
