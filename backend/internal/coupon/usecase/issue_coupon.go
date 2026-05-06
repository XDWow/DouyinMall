package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

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
	OperationID string
}

type IssueCouponOutput struct {
	CouponID int64
}

func (uc *IssueCouponUseCase) Execute(ctx context.Context, input IssueCouponInput) (*IssueCouponOutput, error) {
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if input.TemplateID <= 0 {
		return nil, errors.New("invalid template_id")
	}
	if input.OperationID == "" {
		return nil, errors.New("operation_id is required")
	}

	operation, err := uc.operationRepo.GetByOperationID(ctx, input.OperationID)
	if err != nil && !errors.Is(err, domain.ErrOperationNotFound) {
		return nil, err
	}
	if operation != nil {
		return &IssueCouponOutput{CouponID: operation.UserCouponID}, nil
	}

	template, err := uc.templateRepo.GetByID(ctx, input.TemplateID)
	if err != nil {
		return nil, err
	}
	if !template.CanIssue() {
		return nil, domain.ErrCouponCannotIssue
	}

	count, err := uc.couponRepo.CountByUserAndTemplate(ctx, input.UserID, input.TemplateID)
	if err != nil {
		return nil, err
	}
	if count >= template.PerUserLimit {
		return nil, domain.ErrCouponLimitExceeded
	}

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

	if err := uc.operationRepo.Create(ctx, &domain.CouponOperation{
		OperationID:  input.OperationID,
		UserCouponID: couponID,
		Type:         "ISSUE",
	}); err != nil {
		return nil, err
	}

	if err := uc.templateRepo.IncrIssuedCount(ctx, input.TemplateID); err != nil {
		// Keep the issued coupon result and surface the counter update failure.
		return nil, err
	}

	return &IssueCouponOutput{CouponID: couponID}, nil
}
