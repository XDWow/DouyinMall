package mq

import (
	"context"
	"testing"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/domain"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/rocketmqx"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/stretchr/testify/require"
)

func TestOrderConsumerSkipCommitOnPaidNormalOrder(t *testing.T) {
	repo := &inventoryRepoStub{}
	consumer := NewOrderConsumer(
		nil,
		usecase.NewRefundStockUseCase(repo, logger.NewNopLogger()),
		logger.NewNopLogger(),
		rocketmqx.ConsumerOptions{},
	)

	err := consumer.Consume(nil, OrderStatusUpdateEvent{
		OrderID:   101,
		Status:    orderv1.OrderStatus_ORDER_STATUS_PAID,
		OrderKind: "DIRECT_BUY",
	})

	require.NoError(t, err)
	require.Empty(t, repo.commitCalls)
	require.Empty(t, repo.refundCalls)
}

func TestOrderConsumerRefundStockOnCanceledNormalOrder(t *testing.T) {
	repo := &inventoryRepoStub{}
	consumer := NewOrderConsumer(
		nil,
		usecase.NewRefundStockUseCase(repo, logger.NewNopLogger()),
		logger.NewNopLogger(),
		rocketmqx.ConsumerOptions{},
	)

	err := consumer.Consume(nil, OrderStatusUpdateEvent{
		OrderID:   202,
		Status:    orderv1.OrderStatus_ORDER_STATUS_CANCELED,
		OrderKind: "DIRECT_BUY",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"order_202_refund"}, repo.refundCalls)
	require.Empty(t, repo.commitCalls)
}

func TestOrderConsumerSkipSeckillOrder(t *testing.T) {
	repo := &inventoryRepoStub{}
	consumer := NewOrderConsumer(
		nil,
		usecase.NewRefundStockUseCase(repo, logger.NewNopLogger()),
		logger.NewNopLogger(),
		rocketmqx.ConsumerOptions{},
	)

	err := consumer.Consume(nil, OrderStatusUpdateEvent{
		OrderID:   303,
		Status:    orderv1.OrderStatus_ORDER_STATUS_CANCELED,
		OrderKind: "SECKILL",
	})

	require.NoError(t, err)
	require.Empty(t, repo.commitCalls)
	require.Empty(t, repo.refundCalls)
}

type inventoryRepoStub struct {
	commitCalls []inventoryCommitCall
	refundCalls []string
}

type inventoryCommitCall struct {
	operationID string
	changes     []domain.StockChange
}

func (r *inventoryRepoStub) GetInventory(context.Context, []int64) ([]domain.Inventory, error) {
	return nil, nil
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

func (r *inventoryRepoStub) RefundStock(_ context.Context, operationID string) error {
	r.refundCalls = append(r.refundCalls, operationID)
	return nil
}

func (r *inventoryRepoStub) AdjustStock(context.Context, string, string, []domain.StockChange) error {
	return nil
}
