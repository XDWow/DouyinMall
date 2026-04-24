package mq

import (
	"context"
	"errors"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/stretchr/testify/require"
)

func TestOrderConsumerCommitStockOnPaidNormalOrder(t *testing.T) {
	repo := &inventoryRepoStub{}
	orderCli := &orderClientStub{
		order: &orderv1.Order{
			OrderId:   101,
			OrderKind: "DIRECT_BUY",
			Items: []*orderv1.OrderItem{
				{ProductId: 3001, Quantity: 2},
				{ProductId: 3001, Quantity: 1},
				{ProductId: 3002, Quantity: 4},
			},
		},
	}
	consumer := NewOrderConsumer(
		nil,
		nil,
		usecase.NewCommitStockUseCase(repo, logger.NewNopLogger()),
		usecase.NewReleaseStockUseCase(repo, logger.NewNopLogger()),
		usecase.NewRefundStockUseCase(repo, logger.NewNopLogger()),
		orderCli,
		logger.NewNopLogger(),
	)

	err := consumer.Consume(nil, OrderStatusUpdateEvent{
		OrderID:   101,
		Status:    orderv1.OrderStatus_ORDER_STATUS_PAID,
		OrderKind: "DIRECT_BUY",
	})

	require.NoError(t, err)
	require.Len(t, repo.commitCalls, 1)
	require.Equal(t, "order_101_commit", repo.commitCalls[0].operationID)
	require.ElementsMatch(t, []domain.StockChange{
		{ProductID: 3001, Quantity: -3},
		{ProductID: 3002, Quantity: -4},
	}, repo.commitCalls[0].changes)
}

func TestOrderConsumerSkipSeckillOrder(t *testing.T) {
	repo := &inventoryRepoStub{}
	consumer := NewOrderConsumer(
		nil,
		nil,
		usecase.NewCommitStockUseCase(repo, logger.NewNopLogger()),
		usecase.NewReleaseStockUseCase(repo, logger.NewNopLogger()),
		usecase.NewRefundStockUseCase(repo, logger.NewNopLogger()),
		&orderClientStub{},
		logger.NewNopLogger(),
	)

	err := consumer.Consume(nil, OrderStatusUpdateEvent{
		OrderID:   202,
		Status:    orderv1.OrderStatus_ORDER_STATUS_PAID,
		OrderKind: "SECKILL",
	})

	require.NoError(t, err)
	require.Empty(t, repo.commitCalls)
	require.Empty(t, repo.releaseCalls)
	require.Empty(t, repo.refundCalls)
}

type inventoryRepoStub struct {
	commitCalls  []inventoryCommitCall
	releaseCalls []string
	refundCalls  []string
}

type inventoryCommitCall struct {
	operationID string
	changes     []domain.StockChange
}

func (r *inventoryRepoStub) GetInventory(context.Context, []int64) ([]domain.Inventory, error) {
	return nil, nil
}

func (r *inventoryRepoStub) ReserveStock(context.Context, string, []domain.StockChange, int64) error {
	return nil
}

func (r *inventoryRepoStub) CommitStock(_ context.Context, operationID string, changes []domain.StockChange) error {
	copied := make([]domain.StockChange, len(changes))
	copy(copied, changes)
	r.commitCalls = append(r.commitCalls, inventoryCommitCall{
		operationID: operationID,
		changes:     copied,
	})
	return nil
}

func (r *inventoryRepoStub) ReleaseStock(_ context.Context, operationID string) error {
	r.releaseCalls = append(r.releaseCalls, operationID)
	return nil
}

func (r *inventoryRepoStub) RefundStock(_ context.Context, operationID string) error {
	r.refundCalls = append(r.refundCalls, operationID)
	return nil
}

func (r *inventoryRepoStub) AdjustStock(context.Context, string, string, []domain.StockChange) error {
	return nil
}

type orderClientStub struct {
	order *orderv1.Order
}

func (c *orderClientStub) CreateOrder(context.Context, *orderv1.CreateOrderReq, ...callopt.Option) (*orderv1.CreateOrderResp, error) {
	return nil, errors.New("not implemented")
}

func (c *orderClientStub) ChangeOrderStatus(context.Context, *orderv1.ChangeOrderStatusReq, ...callopt.Option) (*orderv1.ChangeOrderStatusResp, error) {
	return nil, errors.New("not implemented")
}

func (c *orderClientStub) GetOrder(context.Context, *orderv1.GetOrderReq, ...callopt.Option) (*orderv1.GetOrderResp, error) {
	if c.order == nil {
		return &orderv1.GetOrderResp{}, nil
	}
	return &orderv1.GetOrderResp{Order: c.order}, nil
}

func (c *orderClientStub) ListOrder(context.Context, *orderv1.ListOrderReq, ...callopt.Option) (*orderv1.ListOrderResp, error) {
	return nil, errors.New("not implemented")
}

var _ orderservice.Client = (*orderClientStub)(nil)
