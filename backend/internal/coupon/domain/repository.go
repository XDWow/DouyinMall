package domain

import "context"

type CouponTemplateRepository interface {
	GetByID(ctx context.Context, id int64) (CouponTemplate, error)
	IncrIssuedCount(ctx context.Context, id int64) error
}

type CouponRepository interface {
	Issue(ctx context.Context, coupon *Coupon) (int64, error)
	ListByUserID(ctx context.Context, userID int64, status CouponStatus, page, pageSize int) ([]*Coupon, int32, error)
	ListAvailableByUserID(ctx context.Context, userID int64) ([]*Coupon, error)
	GetAvailableByIDs(ctx context.Context, userID int64, couponIDs []int64) ([]*Coupon, error)
	CountByUserAndTemplate(ctx context.Context, userID, templateID int64) (int32, error)
	BatchReserve(ctx context.Context, couponIDs []int64, orderID int64) error
	UpdateStatusByOrderID(ctx context.Context, orderID int64, fromStatus, toStatus CouponStatus) error
	MarkExpiredCoupons(ctx context.Context) (int64, error)
}

type CouponOperationRepository interface {
	Create(ctx context.Context, op *CouponOperation) error
	GetByOperationID(ctx context.Context, operationID string) (*CouponOperation, error)
}
