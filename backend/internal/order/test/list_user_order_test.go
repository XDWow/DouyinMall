package test

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestListUserOrderReturnsErrorForInvalidQuery(t *testing.T) {
	uc := usecase.NewListUserOrderUseCase(&stubStatusOrderRepo{}, logger.NewNopLogger())

	resp, err := uc.Execute(usecase.ListUserOrderCmd{
		UserID: 0,
		Cursor: 0,
		Limit:  10,
	})

	require.Nil(t, resp)
	require.Error(t, err)
}

func TestListUserOrderReturnsNextCursor(t *testing.T) {
	repo := &stubListOrderRepo{
		orders: []*domain.Order{{ID: 9}, {ID: 8}},
		next:   8,
	}
	uc := usecase.NewListUserOrderUseCase(repo, logger.NewNopLogger())

	resp, err := uc.Execute(usecase.ListUserOrderCmd{
		UserID: 1,
		Cursor: 0,
		Limit:  2,
	})

	require.NoError(t, err)
	require.Len(t, resp.Orders, 2)
	require.Equal(t, int64(8), resp.NextCursor)
}

type stubListOrderRepo struct {
	orders []*domain.Order
	next   int64
}

func (s *stubListOrderRepo) Save(context.Context, *domain.Order) error {
	return nil
}

func (s *stubListOrderRepo) FindByID(context.Context, int64) (domain.Order, error) {
	return domain.Order{}, nil
}

func (s *stubListOrderRepo) FindByIDs(context.Context, []int64) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListOrderRepo) FindByIDsForUpdate(context.Context, []int64) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListOrderRepo) UpdateStatus(context.Context, int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (s *stubListOrderRepo) ListOrdersByStatus(context.Context, int64, string) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListOrderRepo) FindExpiredOrders(context.Context, int) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubListOrderRepo) BatchUpdateStatus(context.Context, []int64, domain.OrderStatus, domain.OrderStatus) error {
	return nil
}

func (s *stubListOrderRepo) ListByUserID(context.Context, int64, int64, int) ([]*domain.Order, int64, error) {
	return s.orders, s.next, nil
}


