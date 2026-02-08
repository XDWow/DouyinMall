package usecase

import (
	"context"

	"DouyinMall/internal/coupon/domain"
)

// ==================== 领券用例 ====================

type IssueCouponUseCase struct {
	templateRepo  domain.CouponTemplateRepository
	couponRepo    domain.UserCouponRepository
	operationRepo domain.CouponOperationRepository
}

func NewIssueCouponUseCase(
	templateRepo domain.CouponTemplateRepository,
	couponRepo domain.UserCouponRepository,
	operationRepo domain.CouponOperationRepository,
) *IssueCouponUseCase {
	return &IssueCouponUseCase{
		templateRepo:  templateRepo,
		couponRepo:    couponRepo,
		operationRepo: operationRepo,
	}
}

type IssueCouponInput struct {
	UserID      int64
	TemplateID  int64
	OperationID string // 幂等键（必须带 coupon: 前缀）
}

type IssueCouponOutput struct {
	CouponID int64 // 一次只发一张券
}

func (uc *IssueCouponUseCase) Execute(ctx context.Context, input IssueCouponInput) (*IssueCouponOutput, error) {
	// 1. 幂等检查
	exists, err := uc.operationRepo.Exists(ctx, input.OperationID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrOperationDuplicate
	}

	// 2. 检查模板是否可发放
	template, err := uc.templateRepo.GetByID(ctx, input.TemplateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, domain.ErrCouponTemplateNotFound
	}
	if !template.CanIssue() {
		return nil, domain.ErrCouponCannotIssue
	}

	// 3. 检查用户限领数量
	count, err := uc.couponRepo.CountByUserAndTemplate(ctx, input.UserID, input.TemplateID)
	if err != nil {
		return nil, err
	}
	if count >= template.PerUserLimit {
		return nil, domain.ErrCouponLimitExceeded
	}

	// 4. 创建用户优惠券（一次只发一张）
	validFrom, validTo := template.CalculateValidTime()
	coupon := &domain.UserCoupon{
		UserID:     input.UserID,
		TemplateID: input.TemplateID,
		Status:     domain.UserCouponStatusUnused,
		ValidFrom:  validFrom,
		ValidTo:    validTo,
		Template:   template,
	}
	couponID, err := uc.couponRepo.Create(ctx, coupon)
	if err != nil {
		return nil, err
	}

	// 5. 增加模板已发放数量
	if err := uc.templateRepo.IncrIssuedCount(ctx, input.TemplateID); err != nil {
		// 这里失败不回滚，接受少量超发
		// 或者用分布式事务/Saga
	}

	// 6. 记录操作（幂等）
	_ = uc.operationRepo.Create(ctx, &domain.CouponOperation{
		OperationID:   input.OperationID,
		UserCouponID:  couponID,
		OperationType: "ISSUE",
	})

	return &IssueCouponOutput{CouponID: couponID}, nil
}
