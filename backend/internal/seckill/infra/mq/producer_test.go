package mq

import (
	"context"
	"encoding/json"
	"testing"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/stretchr/testify/require"
)

func TestTransactionListenerMarksSoldOutOnReserveExhausted(t *testing.T) {
	cache := &transactionCacheStub{reserveCode: 1}
	soldOut := &transactionSoldOutMarker{}
	listener := NewTransactionListener(cache, soldOut, logger.NewNopLogger())

	body, err := json.Marshal(seckilldomain.Event{
		RequestNo:  "10001",
		ActivityID: 7,
		UserID:     9,
	})
	require.NoError(t, err)
	msg := primitive.NewMessage(TopicSeckillRequest, body)
	msg.WithProperty(transactionUserTTLProperty, "60")

	state := listener.ExecuteLocalTransaction(msg)

	require.Equal(t, primitive.RollbackMessageState, state)
	require.True(t, soldOut.IsSoldOut(7))
}

type transactionCacheStub struct {
	reserveCode int64
}

func (s *transactionCacheStub) SetActivity(context.Context, *seckilldomain.Activity) error {
	return nil
}

func (s *transactionCacheStub) GetActivity(context.Context, int64) (*seckilldomain.Activity, error) {
	return nil, nil
}

func (s *transactionCacheStub) SetActivityStock(context.Context, int64, int32) error { return nil }

func (s *transactionCacheStub) AtomicReserve(context.Context, int64, int64, string, int64) (int64, error) {
	return s.reserveCode, nil
}

func (s *transactionCacheStub) Compensate(context.Context, int64, int64, string, seckilldomain.Result) error {
	return nil
}

func (s *transactionCacheStub) SetResult(context.Context, seckilldomain.Result) error { return nil }

func (s *transactionCacheStub) GetResult(context.Context, string) (*seckilldomain.Result, error) {
	return nil, nil
}

func (s *transactionCacheStub) ResolveTransaction(context.Context, int64, int64, string) (seckilldomain.TransactionResolution, error) {
	return seckilldomain.TransactionResolutionUnknown, nil
}

type transactionSoldOutMarker struct {
	soldOut map[int64]bool
}

func (m *transactionSoldOutMarker) IsSoldOut(activityID int64) bool {
	return m.soldOut[activityID]
}

func (m *transactionSoldOutMarker) MarkSoldOut(activityID int64) {
	if m.soldOut == nil {
		m.soldOut = make(map[int64]bool)
	}
	m.soldOut[activityID] = true
}

func (m *transactionSoldOutMarker) Clear(activityID int64) {
	if m.soldOut == nil {
		return
	}
	delete(m.soldOut, activityID)
}
