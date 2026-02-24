package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

// ==================== 领券用例 ====================

type IssueCouponUseCase struct {
	templateRepo  domain.CouponTemplateRepository
	couponRepo    domain.CouponRepository
	operationRepo domain.CouponOperationRepository
}

func NewIssueCouponUseCase(
	templateRepo domain.CouponTemplateRepository,
	couponRepo domain.CouponRepository,
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
	// 校验参数
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if input.TemplateID <= 0 {
		return nil, errors.New("invalid template_id")
	}
	if input.OperationID == "" {
		return nil, errors.New("operation_id is required")
	}

	// 1. 幂等检查：查询是否已发放过
	operation, err := uc.operationRepo.GetByOperationID(ctx, input.OperationID)
	if err != nil && err != domain.ErrOperationNotFound {
		return nil, err
	}
	// 如果已发放，直接返回之前的券ID（幂等）
	if operation != nil {
		return &IssueCouponOutput{CouponID: operation.UserCouponID}, nil
	}

	// 2. 检查模板是否可发放
	template, err := uc.templateRepo.GetByID(ctx, input.TemplateID)
	if err != nil {
		return nil, err
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

	// 4. 发放用户优惠券（一次只发一张）
	validFrom, validTo := template.CalculateValidTime()
	coupon := &domain.Coupon{
		UserID:         input.UserID,
		TemplateID:     input.TemplateID,
		Status:         domain.UserCouponStatusUnused,
		ValidStartTime: time.Unix(validFrom, 0),
		ValidEndTime:   time.Unix(validTo, 0),
		Template:       &template,
	}
	couponID, err := uc.couponRepo.Issue(ctx, coupon)
	if err != nil {
		return nil, err
	}

	// 5. 记录幂等操作（必须成功，否则影响幂等性）
	if err := uc.operationRepo.Create(ctx, &domain.CouponOperation{
		OperationID:  input.OperationID,
		UserCouponID: couponID,
		Type:         "ISSUE",
	}); err != nil {
		// TODO: 这里失败应该回滚券的发放，实际应该在事务中完成
		// 当前实现：如果失败，下次重试时会因为券已存在导致重复发券
		return nil, err
	}

	// 6. 增加模板已发放数量（非强一致，允许少量超发）
	if err := uc.templateRepo.IncrIssuedCount(ctx, input.TemplateID); err != nil {
		// 这里失败不回滚券的发放，接受少量超发
		// 因为计数不准确的影响 < 发券失败的影响
	}

	return &IssueCouponOutput{CouponID: couponID}, nil
}
