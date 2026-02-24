package domain

import "context"

type CouponTemplateRepository interface {
	GetByID(ctx context.Context, id int64) (CouponTemplate, error)
	// 增加已发放数量（原子操作）
	IncrIssuedCount(ctx context.Context, id int64) error
}

type CouponRepository interface {
	// 发放优惠券
	Issue(ctx context.Context, coupon *Coupon) (int64, error)
	// 分页查询用户优惠券
	ListByUserID(ctx context.Context, userID int64, status CouponStatus, page, pageSize int) ([]*Coupon, int32, error)
	// 查询用户可用优惠券
	ListAvailableByUserID(ctx context.Context, userID int64) ([]*Coupon, error)
	// 根据指定ID查询可用券（验证并过滤：状态、有效期、归属）
	GetAvailableByIDs(ctx context.Context, userID int64, couponIDs []int64) ([]*Coupon, error)
	// 统计用户已领取某模板的数量
	CountByUserAndTemplate(ctx context.Context, userID, templateID int64) (int32, error)

	// 批量预占优惠券（Unused → Locked）
	BatchReserve(ctx context.Context, couponIDs []int64, orderID int64) error
	// 根据订单ID更新状态，使用场景：
	//   - Commit:  fromStatus=Locked, toStatus=Used
	//   - Release: fromStatus=Locked, toStatus=Unused
	//   - Refund:  fromStatus=Used, toStatus=Unused
	UpdateStatusByOrderID(ctx context.Context, orderID int64, fromStatus, toStatus CouponStatus) error

	// 批量标记过期优惠券（返回影响行数，用于日志）
	MarkExpiredCoupons(ctx context.Context) (int64, error)
}

// 发券幂等
type CouponOperationRepository interface {
	// 创建操作记录（唯一索引保证幂等）
	Create(ctx context.Context, op *CouponOperation) error
	// 根据幂等键查询操作记录，返回已发放的券ID（用于幂等重试）
	GetByOperationID(ctx context.Context, operationID string) (*CouponOperation, error)
}
