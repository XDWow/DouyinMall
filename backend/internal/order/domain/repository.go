package domain

import (
	"context"
)

// OrderRepository 描述业务对持久化的期望，重点在业务语义而非具体存储。
// 接口归属 domain 层；由 infra 提供实现。
// 这样用例层可以在没有真实数据库的情况下完成建模与单测。
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, orderID int64) (Order, error)
	FindByIDs(ctx context.Context, orderIDs []int64) ([]*Order, error)
	FindByIDsForUpdate(ctx context.Context, orderIDs []int64) ([]*Order, error)
	UpdateStatus(ctx context.Context, orderID int64, fromStatus, toStatus OrderStatus) error
	ListOrdersByStatus(ctx context.Context, userID int64, status string) ([]*Order, error)
	// 查找超过 30 分钟未支付的待支付订单（过期）
	FindExpiredOrders(ctx context.Context, limit int) ([]*Order, error)
	// 批量更新订单状态；当前用于批量取消：created -> canceled
	BatchUpdateStatus(ctx context.Context, orderIDs []int64, fromStatus, toStatus OrderStatus) error

	// Keyset 分页：cursor 为上一页最后一条 orderID；首次查询传 0
	// 返回 orders 与 nextCursor（供下一页查询；0 表示没有更多数据）
	ListByUserID(ctx context.Context, userID int64, cursor int64, limit int) (orders []*Order, nextCursor int64, err error)
}

type OutboxRepository interface {
	Add(ctx context.Context, eventType string, payload any) (int64, error)
	BatchAdd(ctx context.Context, eventType string, payloads []any) ([]int64, error)
	// ListPending 分页查询待发送事件
	ListPending(ctx context.Context, offset, limit int) ([]OutboxEvent, error)
	MarkSent(ctx context.Context, id int64) error
	BatchMarkSent(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, id int64) error
	IncreaseRetry(ctx context.Context, id int64) (int, error)
}
