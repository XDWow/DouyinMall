package test

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderSchedulesTimeout(t *testing.T) {
	repo := &stubOrderRepo{}
	delayQueue := &stubDelayQueue{}
	uc := usecase.NewCreateOrderUseCase(repo, delayQueue, logger.NewNopLogger())

	orderID, err := uc.Execute(context.Background(), usecase.CreateOrderCmd{
		OrderID:       10001,
		UserID:        20001,
		Currency:      "CNY",
		Remark:        "leave at door",
		OrderKind:     domain.OrderKindDirectBuy,
		PayableAmount: 79,
		Items: []domain.OrderItem{{
			ProductID:        30001,
			SKUID:            40001,
			Quantity:         1,
			SnapshotPrice:    99,
			SnapshotCurrency: "CNY",
			Price:            99,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, int64(10001), orderID)
	require.Eventually(t, func() bool {
		return delayQueue.orderID == 10001
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, "leave at door", repo.savedOrder.Remark)
	require.Equal(t, int64(99), repo.savedOrder.TotalAmount.Total)
	require.Equal(t, int64(79), repo.savedOrder.PayableAmount.Total)
	require.Equal(t, int64(20), repo.savedOrder.DiscountAmount.Total)
	require.WithinDuration(t, repo.savedOrder.ExpireAt, delayQueue.expireAt, time.Second)
}

type stubOrderRepo struct {
	savedOrder domain.Order
}

func (s *stubOrderRepo) Save(_ context.Context, order *domain.Order) error {
	s.savedOrder = *order
	return nil
}

func (s *stubOrderRepo) FindByID(context.Context, int64) (domain.Order, error) {
	return domain.Order{}, nil
}

func (s *stubOrderRepo) FindByIDs(context.Context, []int64) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubOrderRepo) FindByIDsForUpdate(context.Context, []int64) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubOrderRepo) UpdateStatus(context.Context, int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (s *stubOrderRepo) ListOrdersByStatus(context.Context, int64, string) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubOrderRepo) FindExpiredOrders(context.Context, int) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubOrderRepo) BatchUpdateStatus(context.Context, []int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (s *stubOrderRepo) ListByUserID(context.Context, int64, int64, int) ([]*domain.Order, int64, error) {
	return nil, 0, nil
}

type stubDelayQueue struct {
	orderID  int64
	expireAt time.Time
}

func (s *stubDelayQueue) Enqueue(_ context.Context, orderID int64, expireAt time.Time) error {
	s.orderID = orderID
	s.expireAt = expireAt
	return nil
}

func (s *stubDelayQueue) DrainDue(context.Context, time.Time) ([]int64, error) {
	return nil, nil
}


