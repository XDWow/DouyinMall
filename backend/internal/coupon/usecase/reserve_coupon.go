package usecase

import (
	"context"

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
	if len(input.CouponIDs) == 0 {
		return &ReserveCouponOutput{Success: true, ReservedCount: 0}, nil
	}
	// 订单已锁定，商品和券规则都不会变，唯一可能变化的是券的状态（被用了/过期/锁定），所以只需查的时候再次验证状态
	// BatchGetAvailableByIDs 已经过滤了状态和有效期，这里只需预扣
	// 1. 批量查询指定 ID 的可用券（自动过滤：状态、有效期、归属）
	// 保留 UserID 防止恶意用户传入别人的券 ID
	availableCoupons, err := uc.couponRepo.BatchGetAvailableByIDs(ctx, input.UserID, input.CouponIDs)
	if err != nil {
		return nil, err
	}

	// 2. 批量预扣
	for _, coupon := range availableCoupons {
		if err := coupon.Reserve(input.OrderID); err != nil {
			return nil, err // 理论上不会失败，因为已经是可用券
		}
	}

	if len(availableCoupons) == 0 {
		return &ReserveCouponOutput{Success: true, ReservedCount: 0}, nil
	}

	// 3. 批量持久化（事务内批量更新，失败自动回滚，避免订单失败，但券却被扣了一部分，也不用手动回滚）
	if err := uc.couponRepo.BatchUpdateStatus(ctx, availableCoupons); err != nil {
		return nil, err
	}

	return &ReserveCouponOutput{Success: true, ReservedCount: len(availableCoupons)}, nil
}
