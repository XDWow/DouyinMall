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
	CouponIDs []int64
	OrderID   int64
}

type ReserveCouponOutput struct {
	Success       bool
	ReservedCount int
	Failures      []ReserveCouponFailure
}

type ReserveCouponFailure struct {
	CouponID int64
	Reason   string
}

func (uc *ReserveCouponUseCase) Execute(ctx context.Context, input ReserveCouponInput) (*ReserveCouponOutput, error) {
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if input.OrderID <= 0 {
		return nil, errors.New("invalid order_id")
	}

	couponIDs, ok := normalizeCouponIDs(input.CouponIDs)
	if !ok || len(couponIDs) == 0 {
		return nil, errors.New("invalid coupon_id")
	}

	availableCoupons, err := uc.couponRepo.GetAvailableByIDs(ctx, input.UserID, couponIDs)
	if err != nil {
		return nil, err
	}

	availableSet := make(map[int64]struct{}, len(availableCoupons))
	for _, coupon := range availableCoupons {
		availableSet[coupon.ID] = struct{}{}
	}

	failures := make([]ReserveCouponFailure, 0)
	for _, couponID := range couponIDs {
		if _, exists := availableSet[couponID]; exists {
			continue
		}
		failures = append(failures, ReserveCouponFailure{
			CouponID: couponID,
			Reason:   domain.ErrCouponNotAvailable.Error(),
		})
	}
	if len(failures) > 0 {
		return &ReserveCouponOutput{
			Success:  false,
			Failures: failures,
		}, nil
	}

	if err := uc.couponRepo.BatchReserve(ctx, couponIDs, input.OrderID); err != nil {
		if errors.Is(err, domain.ErrCouponNotAvailable) {
			failures = make([]ReserveCouponFailure, 0, len(couponIDs))
			for _, couponID := range couponIDs {
				failures = append(failures, ReserveCouponFailure{
					CouponID: couponID,
					Reason:   "coupon status changed, please retry",
				})
			}
			return &ReserveCouponOutput{
				Success:  false,
				Failures: failures,
			}, nil
		}
		return nil, err
	}

	return &ReserveCouponOutput{
		Success:       true,
		ReservedCount: len(couponIDs),
	}, nil
}

func normalizeCouponIDs(ids []int64) ([]int64, bool) {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, false
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, true
}


