package mq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	pushconsumer "github.com/apache/rocketmq-client-go/v2/consumer"
	pushprimitive "github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/stretchr/testify/require"
)

func TestOrderStatusConsumerSubscribe(t *testing.T) {
	fake := &fakePushConsumer{}
	c := &OrderStatusConsumer{
		consumer: fake,
		logger:   logger.NewNopLogger(),
		options:  SeckillConsumerOptions{}.withDefaults(),
	}

	err := c.subscribe()
	require.NoError(t, err)
	require.Equal(t, TopicOrderStatusUpdate, fake.subscribeTopic)
	require.Equal(t, pushconsumer.TAG, fake.subscribeSelector.Type)
	require.Equal(t, "*", fake.subscribeSelector.Expression)
	require.NotNil(t, fake.callback)
}

func TestOrderStatusConsumerRetryLaterOnProcessError(t *testing.T) {
	c := &OrderStatusConsumer{
		logger: logger.NewNopLogger(),
		options: SeckillConsumerOptions{
			HandleTimeout: time.Second,
		}.withDefaults(),
		requestRepo: &failingOrderStatusRequestRepo{err: errors.New("db down")},
		cache:       &noopOrderStatusCache{},
	}

	body, err := json.Marshal(OrderStatusUpdateEvent{
		OrderID:   10001,
		Status:    orderv1.OrderStatus_ORDER_STATUS_CANCELED,
		OrderKind: "SECKILL",
	})
	require.NoError(t, err)

	result, consumeErr := c.consumeMessages(context.Background(), &pushprimitive.MessageExt{
		Message:        pushprimitive.Message{Body: body},
		MsgId:          "order-status-msg-1",
		ReconsumeTimes: 1,
	})
	require.NoError(t, consumeErr)
	require.Equal(t, pushconsumer.ConsumeRetryLater, result)
}

func TestOrderStatusConsumerSkipsPoisonMessage(t *testing.T) {
	called := false
	c := &OrderStatusConsumer{
		logger: logger.NewNopLogger(),
		options: SeckillConsumerOptions{
			HandleTimeout: time.Second,
		}.withDefaults(),
		requestRepo: &noopOrderStatusRequestRepo{
			onClose: func() { called = true },
		},
		cache: &noopOrderStatusCache{},
	}

	result, consumeErr := c.consumeMessages(context.Background(), &pushprimitive.MessageExt{
		Message: pushprimitive.Message{Body: []byte("{")},
		MsgId:   "bad-order-status-msg",
	})
	require.NoError(t, consumeErr)
	require.Equal(t, pushconsumer.ConsumeSuccess, result)
	require.False(t, called)
}

func TestOrderStatusConsumerClearsSoldOutMarkerAfterCompensation(t *testing.T) {
	marker := &orderStatusSoldOutMarker{soldOut: map[int64]bool{1: true}}
	c := &OrderStatusConsumer{
		logger:  logger.NewNopLogger(),
		options: SeckillConsumerOptions{HandleTimeout: time.Second}.withDefaults(),
		requestRepo: &closingOrderStatusRequestRepo{
			req: &seckilldomain.Request{
				RequestNo:  "10001",
				ActivityID: 1,
				UserID:     9,
				Status:     seckilldomain.RequestStatusFailed,
				FailReason: seckilldomain.FailReasonOrderCanceled,
			},
		},
		cache:   &noopOrderStatusCache{},
		soldOut: marker,
	}

	err := c.consume(context.Background(), OrderStatusUpdateEvent{
		OrderID:   10001,
		Status:    orderv1.OrderStatus_ORDER_STATUS_CANCELED,
		OrderKind: "SECKILL",
	})

	require.NoError(t, err)
	require.False(t, marker.IsSoldOut(1))
}

type failingOrderStatusRequestRepo struct {
	err error
}

func (r *failingOrderStatusRequestRepo) Create(context.Context, *seckilldomain.Request) error {
	return nil
}
func (r *failingOrderStatusRequestRepo) FindByRequestNo(context.Context, string) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *failingOrderStatusRequestRepo) FindByActivityUser(context.Context, int64, int64) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *failingOrderStatusRequestRepo) AdvanceProcessing(context.Context, seckilldomain.Event) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *failingOrderStatusRequestRepo) CompleteOrderCreating(context.Context, seckilldomain.Event) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *failingOrderStatusRequestRepo) RollbackOrderCreating(context.Context, seckilldomain.Event, string) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *failingOrderStatusRequestRepo) CloseByOrderResult(context.Context, string, string) (*seckilldomain.Request, bool, error) {
	return nil, false, r.err
}
func (r *failingOrderStatusRequestRepo) MarkFail(context.Context, string, string) error { return nil }

type noopOrderStatusRequestRepo struct {
	onClose func()
}

func (r *noopOrderStatusRequestRepo) Create(context.Context, *seckilldomain.Request) error {
	return nil
}
func (r *noopOrderStatusRequestRepo) FindByRequestNo(context.Context, string) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *noopOrderStatusRequestRepo) FindByActivityUser(context.Context, int64, int64) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *noopOrderStatusRequestRepo) AdvanceProcessing(context.Context, seckilldomain.Event) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *noopOrderStatusRequestRepo) CompleteOrderCreating(context.Context, seckilldomain.Event) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *noopOrderStatusRequestRepo) RollbackOrderCreating(context.Context, seckilldomain.Event, string) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *noopOrderStatusRequestRepo) CloseByOrderResult(context.Context, string, string) (*seckilldomain.Request, bool, error) {
	if r.onClose != nil {
		r.onClose()
	}
	return nil, false, nil
}
func (r *noopOrderStatusRequestRepo) MarkFail(context.Context, string, string) error { return nil }

type closingOrderStatusRequestRepo struct {
	req *seckilldomain.Request
}

func (r *closingOrderStatusRequestRepo) Create(context.Context, *seckilldomain.Request) error {
	return nil
}
func (r *closingOrderStatusRequestRepo) FindByRequestNo(context.Context, string) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *closingOrderStatusRequestRepo) FindByActivityUser(context.Context, int64, int64) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *closingOrderStatusRequestRepo) AdvanceProcessing(context.Context, seckilldomain.Event) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *closingOrderStatusRequestRepo) CompleteOrderCreating(context.Context, seckilldomain.Event) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *closingOrderStatusRequestRepo) RollbackOrderCreating(context.Context, seckilldomain.Event, string) (*seckilldomain.Request, error) {
	return nil, nil
}
func (r *closingOrderStatusRequestRepo) CloseByOrderResult(context.Context, string, string) (*seckilldomain.Request, bool, error) {
	return r.req, true, nil
}
func (r *closingOrderStatusRequestRepo) MarkFail(context.Context, string, string) error { return nil }

type noopOrderStatusCache struct{}

func (c *noopOrderStatusCache) SetActivity(context.Context, *seckilldomain.Activity) error {
	return nil
}
func (c *noopOrderStatusCache) GetActivity(context.Context, int64) (*seckilldomain.Activity, error) {
	return nil, nil
}
func (c *noopOrderStatusCache) SetActivityStock(context.Context, int64, int32) error { return nil }
func (c *noopOrderStatusCache) AtomicReserve(context.Context, int64, int64, string, int64) (int64, error) {
	return 0, nil
}
func (c *noopOrderStatusCache) Compensate(context.Context, int64, int64, string, seckilldomain.Result) error {
	return nil
}
func (c *noopOrderStatusCache) SetResult(context.Context, seckilldomain.Result) error { return nil }
func (c *noopOrderStatusCache) GetResult(context.Context, string) (*seckilldomain.Result, error) {
	return nil, nil
}
func (c *noopOrderStatusCache) ResolveTransaction(context.Context, int64, int64, string) (seckilldomain.TransactionResolution, error) {
	return seckilldomain.TransactionResolutionUnknown, nil
}

type orderStatusSoldOutMarker struct {
	soldOut map[int64]bool
}

func (m *orderStatusSoldOutMarker) IsSoldOut(activityID int64) bool {
	return m.soldOut[activityID]
}

func (m *orderStatusSoldOutMarker) MarkSoldOut(activityID int64) {
	if m.soldOut == nil {
		m.soldOut = make(map[int64]bool)
	}
	m.soldOut[activityID] = true
}

func (m *orderStatusSoldOutMarker) Clear(activityID int64) {
	if m.soldOut == nil {
		return
	}
	delete(m.soldOut, activityID)
}
