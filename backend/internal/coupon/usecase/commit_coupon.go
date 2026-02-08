package usecase

import (
	"context"

	"DouyinMall/internal/coupon/domain"
)

// ==================== 确认使用优惠券用例 ====================

type CommitCouponUseCase struct {
	couponRepo domain.UserCouponRepository
}

func NewCommitCouponUseCase(couponRepo domain.UserCouponRepository) *CommitCouponUseCase {
	return &CommitCouponUseCase{couponRepo: couponRepo}
}

type CommitCouponInput struct {
	OrderID int64
}

func (uc *CommitCouponUseCase) Execute(ctx context.Context, input CommitCouponInput) error {
	// 1. 根据订单查询被锁定的优惠券
	coupon, err := uc.couponRepo.GetByOrderID(ctx, input.OrderID, true)
	if err != nil {
		return err
	}
	if coupon == nil {
		// 没有优惠券需要确认，幂等成功
		return nil
	}

	// 2. 幂等检查：如果已经是 Used 状态，直接返回成功
	if coupon.Status == domain.UserCouponStatusUsed {
		return nil
	}

	// 3. 调用领域方法确认（内部会检查状态必须是 Locked）
	if err := coupon.Commit(); err != nil {
		return err
	}

	// 4. 持久化（关键：WHERE status=Locked AND order_id=? 保证幂等）
	if err := uc.couponRepo.UpdateStatus(ctx, coupon); err != nil {
		return err
	}

	return nil
}
