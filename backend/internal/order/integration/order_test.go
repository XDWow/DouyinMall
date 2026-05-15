//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderUseCase(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryOrderRepo()
	log := logger.NewNopLogger()
	createOrderUC := usecase.NewCreateOrderUseCase(repo, noopDelayQueue{}, log)

	orderID, err := createOrderUC.Execute(ctx, usecase.CreateOrderCmd{
		OrderID:    10001,
		UserID:     20001,
		Currency:   "CNY",
		Remark:     "integration remark",
		OrderKind:  domain.OrderKindSeckill,
		ActivityID: 30001,
		Address: domain.Address{
			City: "Shanghai",
		},
		Items: []domain.OrderItem{{
			ProductID:        40001,
			SKUID:            50001,
			Quantity:         1,
			SnapshotPrice:    9900,
			SnapshotCurrency: "CNY",
			Price:            9900,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10001), orderID)

	order, err := repo.FindByID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusCreated, order.Status)
	assert.Equal(t, domain.OrderKindSeckill, order.OrderKind)
	assert.Equal(t, int64(30001), order.ActivityID)
	assert.Equal(t, "integration remark", order.Remark)
	assert.Equal(t, int64(9900), order.PayableAmount.Total)
	require.Len(t, order.OrderItems, 1)
	assert.Equal(t, int64(50001), order.OrderItems[0].SKUID)
	assert.WithinDuration(t, time.Now().Add(30*time.Minute), order.ExpireAt, 5*time.Second)
}

type memoryOrderRepo struct {
	orders map[int64]domain.Order
}

func newMemoryOrderRepo() *memoryOrderRepo {
	return &memoryOrderRepo{orders: make(map[int64]domain.Order)}
}

func (r *memoryOrderRepo) Save(_ context.Context, order *domain.Order) error {
	r.orders[order.ID] = *order
	return nil
}

func (r *memoryOrderRepo) FindByID(_ context.Context, orderID int64) (domain.Order, error) {
	order, ok := r.orders[orderID]
	if !ok {
		return domain.Order{}, domain.ErrRecordNotFound
	}
	return order, nil
}

func (r *memoryOrderRepo) FindByIDs(_ context.Context, orderIDs []int64) ([]*domain.Order, error) {
	orders := make([]*domain.Order, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order, ok := r.orders[orderID]
		if !ok {
			return nil, domain.ErrRecordNotFound
		}
		orderCopy := order
		orders = append(orders, &orderCopy)
	}
	return orders, nil
}

func (r *memoryOrderRepo) FindByIDsForUpdate(_ context.Context, orderIDs []int64) ([]*domain.Order, error) {
	return r.FindByIDs(context.Background(), orderIDs)
}

func (r *memoryOrderRepo) UpdateStatus(context.Context, int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (r *memoryOrderRepo) ListOrdersByStatus(context.Context, int64, string) ([]*domain.Order, error) {
	return nil, nil
}

func (r *memoryOrderRepo) FindExpiredOrders(context.Context, int) ([]*domain.Order, error) {
	return nil, nil
}

func (r *memoryOrderRepo) BatchUpdateStatus(context.Context, []int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (r *memoryOrderRepo) ListByUserID(context.Context, int64, int64, int) ([]*domain.Order, int64, error) {
	return nil, 0, nil
}

type noopDelayQueue struct{}

func (noopDelayQueue) Enqueue(context.Context, int64, time.Time) error { return nil }

func (noopDelayQueue) DrainDue(context.Context, time.Time) ([]int64, error) {
	return nil, nil
}
