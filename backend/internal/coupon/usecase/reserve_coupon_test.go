package usecase

import (
	"context"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/stretchr/testify/require"
)

func TestReserveCouponReturnsFailuresForUnavailableCoupons(t *testing.T) {
	repo := &stubCouponRepository{
		availableCoupons: []*domain.Coupon{
			{ID: 1, UserID: 100},
		},
	}
	uc := NewReserveCouponUseCase(repo)

	output, err := uc.Execute(context.Background(), ReserveCouponInput{
		UserID:    100,
		CouponIDs: []int64{1, 2},
		OrderID:   200,
	})
	require.NoError(t, err)
	require.False(t, output.Success)
	require.Len(t, output.Failures, 1)
	require.Equal(t, int64(2), output.Failures[0].CouponID)
	require.False(t, repo.batchReserveCalled)
}

func TestReserveCouponDoesNotDuplicateReserveSameCouponID(t *testing.T) {
	repo := &stubCouponRepository{
		availableCoupons: []*domain.Coupon{
			{ID: 1, UserID: 100},
		},
	}
	uc := NewReserveCouponUseCase(repo)

	output, err := uc.Execute(context.Background(), ReserveCouponInput{
		UserID:    100,
		CouponIDs: []int64{1, 1},
		OrderID:   200,
	})
	require.NoError(t, err)
	require.True(t, output.Success)
	require.Equal(t, 1, output.ReservedCount)
	require.True(t, repo.batchReserveCalled)
	require.Equal(t, []int64{1}, repo.batchReserveIDs)
}

type stubCouponRepository struct {
	availableCoupons   []*domain.Coupon
	batchReserveCalled bool
	batchReserveIDs    []int64
}

func (s *stubCouponRepository) Issue(context.Context, *domain.Coupon) (int64, error) {
	panic("unexpected call")
}

func (s *stubCouponRepository) ListByUserID(context.Context, int64, domain.CouponStatus, int, int) ([]*domain.Coupon, int32, error) {
	panic("unexpected call")
}

func (s *stubCouponRepository) ListAvailableByUserID(context.Context, int64) ([]*domain.Coupon, error) {
	panic("unexpected call")
}

func (s *stubCouponRepository) GetAvailableByIDs(context.Context, int64, []int64) ([]*domain.Coupon, error) {
	return s.availableCoupons, nil
}

func (s *stubCouponRepository) CountByUserAndTemplate(context.Context, int64, int64) (int32, error) {
	panic("unexpected call")
}

func (s *stubCouponRepository) BatchReserve(_ context.Context, couponIDs []int64, orderID int64) error {
	s.batchReserveCalled = true
	s.batchReserveIDs = append([]int64(nil), couponIDs...)
	return nil
}

func (s *stubCouponRepository) UpdateStatusByOrderID(context.Context, int64, domain.CouponStatus, domain.CouponStatus) error {
	panic("unexpected call")
}

func (s *stubCouponRepository) MarkExpiredCoupons(context.Context) (int64, error) {
	panic("unexpected call")
}
