package usecase

import (
	"context"
	"errors"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

type ReserveCouponUseCase struct {
	couponRepo domain.CouponRepository
}

func NewReserveCouponUseCase(couponRepo domain.CouponRepository) *ReserveCouponUseCase {
	return &ReserveCouponUseCase{couponRepo: couponRepo}
}

type ReserveCouponInput struct {
	UserID    int64
	CouponIDs []int64 // 支持多张券
	OrderID   int64
}

type ReserveCouponOutput struct {
	Success       bool
	ReservedCount int // 成功预扣的券数量
}

func (uc *ReserveCouponUseCase) Execute(ctx context.Context, input ReserveCouponInput) (*ReserveCouponOutput, error) {
	// 校验参数
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if input.OrderID <= 0 {
		return nil, errors.New("invalid order_id")
	}
	if len(input.CouponIDs) == 0 || input.CouponIDs[0] <= 0 {
		return nil, errors.New("invalid coupon_id")
	}

	if len(input.CouponIDs) == 0 {
		return &ReserveCouponOutput{Success: true, ReservedCount: 0}, nil
	}
	// 订单已锁定，商品和券规则都不会变，唯一可能变化的是券的状态（被用了/过期/锁定），所以只需查的时候再次验证状态
	// GetAvailableByIDs 已经过滤了状态和有效期，这里只需预扣
	// 1. 验证指定 ID 的券是否可用（自动过滤：状态、有效期、归属）
	// 保留 UserID 防止恶意用户传入别人的券 ID
	availableCoupons, err := uc.couponRepo.GetAvailableByIDs(ctx, input.UserID, input.CouponIDs)
	if err != nil {
		return nil, err
	}

	if len(availableCoupons) == 0 {
		return &ReserveCouponOutput{Success: true, ReservedCount: 0}, nil
	}

	// 2. 提取券ID
	couponIDs := make([]int64, 0, len(availableCoupons))
	for _, coupon := range availableCoupons {
		couponIDs = append(couponIDs, coupon.ID)
	}

	// 3. 批量预扣（事务内批量更新，失败自动回滚）
	if err := uc.couponRepo.BatchReserve(ctx, couponIDs, input.OrderID); err != nil {
		return nil, err
	}

	return &ReserveCouponOutput{Success: true, ReservedCount: len(couponIDs)}, nil
}
