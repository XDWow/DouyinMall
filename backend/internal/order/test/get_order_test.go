package test

import (
	"context"
	"errors"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestGetOrderInvalidID(t *testing.T) {
	repo := &stubStatusOrderRepo{order: domain.Order{ID: 1}}
	uc := usecase.NewGetOrderUseCase(repo, logger.NewNopLogger())

	_, err := uc.Execute(context.Background(), usecase.GetOrderCmd{OrderID: 0})
	require.Error(t, err)

	_, err = uc.Execute(context.Background(), usecase.GetOrderCmd{OrderID: -1})
	require.Error(t, err)
}

func TestGetOrderSuccess(t *testing.T) {
	want := domain.Order{ID: 9001, UserID: 1, Status: domain.OrderStatusPaid}
	repo := &stubStatusOrderRepo{order: want}
	uc := usecase.NewGetOrderUseCase(repo, logger.NewNopLogger())

	got, err := uc.Execute(context.Background(), usecase.GetOrderCmd{OrderID: 9001})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, want.ID, got.ID)
	require.Equal(t, want.Status, got.Status)
}

func TestGetOrderNotFound(t *testing.T) {
	repo := &stubStatusOrderRepo{
		ordersByID: map[int64]domain.Order{},
	}
	uc := usecase.NewGetOrderUseCase(repo, logger.NewNopLogger())

	_, err := uc.Execute(context.Background(), usecase.GetOrderCmd{OrderID: 404})
	require.ErrorIs(t, err, domain.ErrRecordNotFound)
}

func TestGetOrderRepoError(t *testing.T) {
	repo := &stubStatusOrderRepo{findByIDErr: errors.New("db down")}
	uc := usecase.NewGetOrderUseCase(repo, logger.NewNopLogger())

	_, err := uc.Execute(context.Background(), usecase.GetOrderCmd{OrderID: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "db down")
}
