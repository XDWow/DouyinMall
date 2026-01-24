package domain

import (
	"context"
)

// Repository 接口描述的是业务对世界的期望，重点在业务，而不是基础设施
// 所以它属于 domain 层
// infra 只负责提供实现
// 这样业务层才能在没有数据库的情况下被完整建模和测试
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	UpdateStatus(ctx context.Context, order *Order) error
	ListOrdersByStatus(ctx context.Context, userID int64, status string) ([]*Order, error)
	// 查找超过30分钟未支付的待支付订单（过期）
	FindExpiredOrders(ctx context.Context, limit int) ([]*Order, error)
	// 批量更新订单状态，现在只用于批量取消订单：pending -> canceled
	BatchUpdateStatus(ctx context.Context, orderIDs []int64, fromStatus, toStatus OrderStatus) error

	ListByUserID(ctx context.Context, userID int64, offset, limit int) ([]Order, error)
}

type OutboxRepository interface {
	Add(ctx context.Context, eventType string, payload any) error
	BatchAdd(ctx context.Context, eventType string, payloads []any) error
	// ListPending 分页查询待发送的事件
	ListPending(ctx context.Context, offset, limit int) ([]OutboxEvent, error)
	MarkSent(ctx context.Context, id int64) error
	BatchMarkSent(ctx context.Context, ids []int64) error
	MarkFailed(ctx context.Context, id int64) error
	IncreaseRetry(ctx context.Context, id int64) (int, error)
}
