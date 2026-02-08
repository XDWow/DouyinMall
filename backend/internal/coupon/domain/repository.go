package domain

import "context"

// ==================== Repository 接口（依赖倒置） ====================

// CouponTemplateRepository 优惠券模板仓储
type CouponTemplateRepository interface {
	// 创建模板
	Create(ctx context.Context, template *CouponTemplate) (int64, error)
	// 根据ID查询
	GetByID(ctx context.Context, id int64) (*CouponTemplate, error)
	// 增加已发放数量（原子操作）
	IncrIssuedCount(ctx context.Context, id int64) error
}

type CouponRepository interface {
	Create(ctx context.Context, coupon *Coupon) (int64, error)
	// 根据ID查询（带模板信息）
	GetByID(ctx context.Context, id int64) (*Coupon, error)
	// 批量查询（带模板信息）
	BatchGetByIDs(ctx context.Context, ids []int64) ([]*Coupon, error)
	// 批量查询指定 ID 的可用券（过滤：归属、状态=未使用、未过期）
	BatchGetAvailableByIDs(ctx context.Context, userID int64, ids []int64) ([]*Coupon, error)
	// 根据订单ID查询（用于 Commit/Release/Refund）
	GetByOrderID(ctx context.Context, orderID int64, locked bool) (*Coupon, error)
	// 查询用户优惠券列表
	ListByUserID(ctx context.Context, userID int64, status CouponStatus, page, pageSize int) ([]Coupon, int32, error)
	// 查询当前用户可用的所有优惠券
	ListAvailableByUserID(ctx context.Context, userID int64) ([]Coupon, error)
	// 统计用户已领取某模板的数量
	CountByUserAndTemplate(ctx context.Context, userID, templateID int64) (int32, error)
	// 更新状态（预扣/确认/释放/退还）
	// 关键：必须使用状态机 + WHERE 条件防止并发冲突
	// 例如 Reserve: UPDATE ... WHERE id=? AND status=1 (Unused)
	// 例如 Commit:  UPDATE ... WHERE id=? AND status=2 AND order_id=? (Locked)
	UpdateStatus(ctx context.Context, coupon *Coupon) error

	// BatchUpdateStatus 批量更新状态（事务内执行，用于预扣多张券），全部成功或全部失败，失败自动回滚
	BatchUpdateStatus(ctx context.Context, coupons []*Coupon) error

	// ReleaseByOrderID 释放预扣（状态转移：Locked → Unused）
	// 原子操作：UPDATE ... WHERE order_id=? AND status=Locked
	// 返回受影响的行数，0表示没有需要释放的优惠券（幂等）
	ReleaseByOrderID(ctx context.Context, orderID int64) (int64, error)
	// RefundByOrderID 退还优惠券（状态转移：Used → Unused，延长有效期）
	// 原子操作：UPDATE ... WHERE order_id=? AND status=Used
	// 返回受影响的行数，0表示没有需要退还的优惠券（幂等）
	RefundByOrderID(ctx context.Context, orderID int64) (int64, error)
}

// CouponOperationRepository 操作记录仓储（幂等）
type CouponOperationRepository interface {
	// 创建操作记录（唯一索引保证幂等）
	Create(ctx context.Context, op *CouponOperation) error
	// 检查操作是否已存在
	Exists(ctx context.Context, operationID string) (bool, error)
}
