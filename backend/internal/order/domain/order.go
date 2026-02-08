package domain

import "time"

type Order struct {
	ID        int64
	UserID    int64
	Phone     string
	Addr      Address
	Status    OrderStatus
	CreatedAt time.Time
	ExpireAt  time.Time // 用来控制订单创建30分钟未支付自动取消，属于业务层面，出现在domain, usecase

	//Amt        Amount   // 原来实现，加入优惠券后变下面
	TotalAmount    Amount // 商品原价合计
	PayableAmount  Amount // 实付金额
	DiscountAmount Amount // 优惠总额

	OrderItems []OrderItem
}

type OrderItem struct {
	ProductID        int64
	Quantity         int64
	SnapshotPrice    int64
	SnapshotCurrency string
	Price            int64 // 最终真正需要支付的价格，随币种变化
}

type Address struct {
	Street  string
	City    string
	State   string
	Country string
	Zipcode string
}

type Amount struct {
	Currency string
	Total    int64
}

type OrderStatus uint8

func (s OrderStatus) AsUint8() uint8 {
	return uint8(s)
}

const (
	OrderStatusUnknown   OrderStatus = iota
	OrderStatusCreated               // 待支付
	OrderStatusPaid                  // 已支付
	OrderStatusToShip                // 库存已确认，待发货
	OrderStatusShipped               // 已发货
	OrderStatusCompleted             // 已完成（确认收货）
	OrderStatusCanceled              // 已取消（未支付超时）
	OrderStatusRefunded              // 已退款，两种场景：售后退款；库存不够退款
)

// 防止有人觉得支付成功就能发货了，唯一能转为发货的状态：ToShip
func (s OrderStatus) CanShip() bool {
	return s == OrderStatusToShip
}

/*
 订单状态设计这里花了点功夫，还好没出大问题的时候，来最终确定了状态，以及状态转移，这里是要超前设计的
 本来没考虑发货什么的，后面商家会有这些操作，所以引入了发货相关的状态
 演进过程，所以我这里实际是：业务驱动状态：
 方案一：同步确认库存
	已支付 = 支付成功 + 库存已确认
	流程
		支付成功
		→ 同步调用库存服务扣库存
		→ 成功：Order = 待发货
		→ 失败：退款 / 回滚
	优点（简单干净）
		状态机最简单、最干净
		不存在「已支付但不可发货」的订单
		实现简单
	缺点（同步）因为需要库存确认的结果来进行状态转移，所以只能同步
		高并发下，同步调用压力大
		耦合库存服务，支付链路变长，响应慢。支付是用户相关的，库存确认是用户无关，不应该影响用户体验。我只关心付款成功没，库存你自己搞

	适用
		QPS 不高
		SKU 较少

 方案二：异步确认库存
	已支付 = 仅支付成功
	待发货 = 支付成功 + 库存已确认
	流程
		支付成功
		→ Order = 已支付
		→ 发送 MQ
		→ 库存服务异步确认
		   ├─ 成功：Order = 待发货
		   └─ 失败：Order = 退款中 / 已退款
	优点（异步MQ的优点）
		支付链路短、响应快
		解耦，库存服务可独立扩展
		削峰，抗流量峰值能力强

	缺点
		必然存在内部中间态，实现复杂一点
		不能一直中间态吧，需要补偿、超时兜底
		存在消息重复发送？丢失？重复消费？（解决：重复：幂等；丢失：Outbox重试）
	适用
		高并发
		秒杀 / 大促

一句话：同步方案把一致性放在支付链路里，异步方案把一致性放在事后确认里。
*/
