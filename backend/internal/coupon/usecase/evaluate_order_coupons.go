package usecase

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

type EvaluateOrderCouponsInput struct {
	UserID int64
	Items  []domain.OrderItem // 订单商品列表（Checkout 传入，只含商品原价）
}

type EvaluateOrderCouponsOutput struct {
	Coupons []EvaluatedCoupon
}

// 结算页优惠券评估结果
type EvaluatedCoupon struct {
	Coupon domain.Coupon
	Usable bool
	Reason string // 不可用原因
}

// 在结算页中找出所有本订单可用的优惠券，给用户选择
type EvaluateOrderCouponsUseCase struct {
	couponRepo domain.CouponRepository
}

func NewEvaluateOrderCouponsUseCase(
	couponRepo domain.CouponRepository,
) *EvaluateOrderCouponsUseCase {
	return &EvaluateOrderCouponsUseCase{
		couponRepo: couponRepo,
	}
}

func (uc *EvaluateOrderCouponsUseCase) Execute(
	ctx context.Context,
	input EvaluateOrderCouponsInput,
) (*EvaluateOrderCouponsOutput, error) {
	if len(input.Items) == 0 || input.UserID < 0 {
		return nil, fmt.Errorf("invalid input")
	}

	// 查询用户层面可用的优惠券（未使用 && 未过期）
	coupons, err := uc.couponRepo.ListAvailableByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	results := make([]EvaluatedCoupon, 0, len(coupons))
	for _, c := range coupons {
		// 订单层面检查：适用范围 + 门槛
		usable, reason := uc.evaluateCoupon(c, input)
		results = append(results, EvaluatedCoupon{
			Coupon: c,
			Usable: usable,
			Reason: reason,
		})
	}

	return &EvaluateOrderCouponsOutput{Coupons: results}, nil
}

// 订单维度的适用性检查，本来逻辑在这个方法中，后面优化了
// 把逻辑放 domain 层，因为"判断券能否用于订单"是优惠券模板的固有职责
// 符合 DDD 领域驱动设计，业务规则由领域对象负责
func (uc *EvaluateOrderCouponsUseCase) evaluateCoupon(
	c domain.Coupon,
	input EvaluateOrderCouponsInput,
) (bool, string) {
	tpl := c.Template
	if tpl == nil {
		return false, "券模板不存在"
	}

	// 检查适用性（计算适用金额 + 检查门槛）
	return tpl.IsApplicableToOrder(input.Items)
}
