package grpc

import (
	"context"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/stretchr/testify/require"
)

func TestOrderHandlerListOrderReturnsNextCursor(t *testing.T) {
	handler := NewOrderHandler(
		nil,
		nil,
		usecase.NewListUserOrderUseCase(&stubListRepo{
			orders: []*domain.Order{{ID: 100}, {ID: 99}},
			next:   99,
		}, logger.NewNopLogger()),
		nil,
	)

	resp, err := handler.ListOrder(context.Background(), &orderv1.ListOrderReq{
		UserId: 1,
		Cursor: 0,
		Limit:  2,
	})

	require.NoError(t, err)
	require.Len(t, resp.GetOrders(), 2)
	require.Equal(t, int64(99), resp.GetNextCursor())
}

func TestToProtoOrderReturnsPaidStatus(t *testing.T) {
	order := &domain.Order{
		ID:     1,
		UserID: 2,
		Status: domain.OrderStatusPaid,
	}

	resp := toProtoOrder(order)

	require.Equal(t, orderv1.OrderStatus_ORDER_STATUS_PAID, resp.GetOrderStatus())
}

type stubListRepo struct {
	orders []*domain.Order
	next   int64
}

func (s *stubListRepo) Save(context.Context, *domain.Order) error { return nil }

func (s *stubListRepo) FindByID(context.Context, int64) (domain.Order, error) {
	return domain.Order{}, nil
}

func (s *stubListRepo) FindByIDs(context.Context, []int64) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListRepo) FindByIDsForUpdate(context.Context, []int64) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListRepo) UpdateStatus(context.Context, int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (s *stubListRepo) ListOrdersByStatus(context.Context, int64, string) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListRepo) FindExpiredOrders(context.Context, int) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListRepo) BatchUpdateStatus(context.Context, []int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (s *stubListRepo) ListByUserID(context.Context, int64, int64, int) ([]*domain.Order, int64, error) {
	return s.orders, s.next, nil
}
