package test

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestChangeOrderStatusMarksOutboxByOutboxID(t *testing.T) {
	outboxRepo := &stubOutboxRepo{addID: 42}
	orderRepo := &stubStatusOrderRepo{
		order: domain.Order{ID: 1001, UserID: 2001, Status: domain.OrderStatusCreated},
	}
	producer := mq.NewSaramaProducer(&stubSyncProducer{})
	uc := usecase.NewChangeOrderStatusUseCase(orderRepo, outboxRepo, producer, stubTxManager{}, logger.NewNopLogger())

	result, err := uc.Execute(context.Background(), usecase.ChangeOrderStatusCmd{
		OrderID: 1001,
		Action:  domain.OrderActionPay,
	})
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Eventually(t, func() bool {
		outboxRepo.mu.Lock()
		defer outboxRepo.mu.Unlock()
		return len(outboxRepo.markSentIDs) == 1
	}, time.Second, 10*time.Millisecond)

	outboxRepo.mu.Lock()
	defer outboxRepo.mu.Unlock()
	require.Equal(t, []int64{42}, outboxRepo.markSentIDs)
	require.Empty(t, outboxRepo.retryIDs)
}

func TestChangeOrderStatusLeavesPendingOutboxOnFastPathSendFailure(t *testing.T) {
	outboxRepo := &stubOutboxRepo{addID: 88}
	orderRepo := &stubStatusOrderRepo{
		order: domain.Order{ID: 1002, UserID: 2002, Status: domain.OrderStatusCreated},
	}
	producer := mq.NewSaramaProducer(&stubSyncProducer{sendMessageErr: errors.New("send failed")})
	uc := usecase.NewChangeOrderStatusUseCase(orderRepo, outboxRepo, producer, stubTxManager{}, logger.NewNopLogger())

	result, err := uc.Execute(context.Background(), usecase.ChangeOrderStatusCmd{
		OrderID: 1002,
		Action:  domain.OrderActionCancel,
	})
	require.NoError(t, err)
	require.True(t, result.Changed)
	time.Sleep(50 * time.Millisecond)

	outboxRepo.mu.Lock()
	defer outboxRepo.mu.Unlock()
	require.Empty(t, outboxRepo.retryIDs)
	require.Empty(t, outboxRepo.markSentIDs)
}

func TestChangeOrderStatusReturnsUnchangedResult(t *testing.T) {
	outboxRepo := &stubOutboxRepo{}
	orderRepo := &stubStatusOrderRepo{
		order: domain.Order{ID: 1003, UserID: 2003, Status: domain.OrderStatusPaid},
	}
	producer := mq.NewSaramaProducer(&stubSyncProducer{})
	uc := usecase.NewChangeOrderStatusUseCase(orderRepo, outboxRepo, producer, stubTxManager{}, logger.NewNopLogger())

	result, err := uc.Execute(context.Background(), usecase.ChangeOrderStatusCmd{
		OrderID: 1003,
		Action:  domain.OrderActionPay,
	})
	require.NoError(t, err)
	require.False(t, result.Changed)
	require.Zero(t, orderRepo.updatedOrderID)
}

func TestBatchCancelOrderEmptyOrderIDs(t *testing.T) {
	orderRepo := &stubStatusOrderRepo{}
	outboxRepo := &stubOutboxRepo{}
	producer := mq.NewSaramaProducer(&stubSyncProducer{})
	uc := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTxManager{}, logger.NewNopLogger())

	require.NoError(t, uc.Execute(context.Background(), nil))
	require.NoError(t, uc.Execute(context.Background(), []int64{}))
	require.False(t, orderRepo.findByIDsLocked)
}

func TestBatchCancelOrderSkipsNonCreated(t *testing.T) {
	outboxRepo := &stubOutboxRepo{}
	orderRepo := &stubStatusOrderRepo{
		ordersByID: map[int64]domain.Order{
			21: {ID: 21, UserID: 201, Status: domain.OrderStatusPaid},
		},
	}
	producer := mq.NewSaramaProducer(&stubSyncProducer{})
	uc := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTxManager{}, logger.NewNopLogger())

	require.NoError(t, uc.Execute(context.Background(), []int64{21}))
	require.False(t, orderRepo.findByIDsLocked)
	outboxRepo.mu.Lock()
	defer outboxRepo.mu.Unlock()
	require.Empty(t, outboxRepo.batchAddIDs)
	require.Empty(t, outboxRepo.batchMarkSentIDs)
}

func TestBatchCancelOrderMarksSentByOutboxID(t *testing.T) {
	outboxRepo := &stubOutboxRepo{batchAddIDs: []int64{301, 302}}
	orderRepo := &stubStatusOrderRepo{
		ordersByID: map[int64]domain.Order{
			11: {ID: 11, UserID: 101, Status: domain.OrderStatusCreated},
			12: {ID: 12, UserID: 102, Status: domain.OrderStatusCreated},
		},
	}
	producer := mq.NewSaramaProducer(&stubSyncProducer{})
	uc := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTxManager{}, logger.NewNopLogger())

	err := uc.Execute(context.Background(), []int64{11, 12})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		outboxRepo.mu.Lock()
		defer outboxRepo.mu.Unlock()
		return len(outboxRepo.batchMarkSentIDs) == 2
	}, time.Second, 10*time.Millisecond)

	outboxRepo.mu.Lock()
	defer outboxRepo.mu.Unlock()
	require.Equal(t, []int64{301, 302}, outboxRepo.batchMarkSentIDs)
	require.Empty(t, outboxRepo.retryIDs)
	require.False(t, orderRepo.findByIDsLocked)
	require.Equal(t, []int64{11, 12}, orderRepo.batchUpdatedIDs)
}

func TestBatchCancelOrderFailsWhenBatchUpdateStatusFails(t *testing.T) {
	outboxRepo := &stubOutboxRepo{batchAddIDs: []int64{401}}
	orderRepo := &stubStatusOrderRepo{
		ordersByID: map[int64]domain.Order{
			31: {ID: 31, UserID: 301, Status: domain.OrderStatusCreated},
			32: {ID: 32, UserID: 302, Status: domain.OrderStatusCreated},
		},
		batchUpdateErr: domain.ErrRecordNotFound,
	}
	producer := mq.NewSaramaProducer(&stubSyncProducer{})
	uc := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTxManager{}, logger.NewNopLogger())

	err := uc.Execute(context.Background(), []int64{31, 32})
	require.ErrorIs(t, err, domain.ErrRecordNotFound)

	outboxRepo.mu.Lock()
	defer outboxRepo.mu.Unlock()
	require.Empty(t, outboxRepo.batchMarkSentIDs)
	require.Equal(t, []int64{31, 32}, orderRepo.batchUpdatedIDs)
}

type stubStatusOrderRepo struct {
	order               domain.Order
	ordersByID          map[int64]domain.Order
	findByIDErr         error
	findByIDsLocked     bool
	updateStatusErrByID map[int64]error
	updatedOrderID      int64
	updatedOrderIDs     []int64
	updatedFrom         domain.OrderStatus
	updatedTo           domain.OrderStatus
	batchUpdatedIDs     []int64
	batchUpdatedFrom    domain.OrderStatus
	batchUpdatedTo      domain.OrderStatus
	batchUpdateErr      error
}

func (s *stubStatusOrderRepo) Save(context.Context, *domain.Order) error {
	return nil
}

func (s *stubStatusOrderRepo) FindByID(_ context.Context, orderID int64) (domain.Order, error) {
	if s.findByIDErr != nil {
		return domain.Order{}, s.findByIDErr
	}
	if s.ordersByID != nil {
		o, ok := s.ordersByID[orderID]
		if !ok {
			return domain.Order{}, domain.ErrRecordNotFound
		}
		return o, nil
	}
	return s.order, nil
}

func (s *stubStatusOrderRepo) FindByIDs(_ context.Context, orderIDs []int64) ([]*domain.Order, error) {
	return s.findByIDs(orderIDs)
}

func (s *stubStatusOrderRepo) FindByIDsForUpdate(_ context.Context, orderIDs []int64) ([]*domain.Order, error) {
	s.findByIDsLocked = true
	return s.findByIDs(orderIDs)
}

func (s *stubStatusOrderRepo) findByIDs(orderIDs []int64) ([]*domain.Order, error) {
	orders := make([]*domain.Order, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		if s.ordersByID != nil {
			order, ok := s.ordersByID[orderID]
			if !ok {
				return nil, domain.ErrRecordNotFound
			}
			orderCopy := order
			orders = append(orders, &orderCopy)
			continue
		}
		orderCopy := s.order
		orderCopy.ID = orderID
		orders = append(orders, &orderCopy)
	}
	return orders, nil
}

func (s *stubStatusOrderRepo) UpdateStatus(_ context.Context, orderID int64, fromStatus, toStatus domain.OrderStatus) error {
	s.updatedOrderID = orderID
	s.updatedOrderIDs = append(s.updatedOrderIDs, orderID)
	s.updatedFrom = fromStatus
	s.updatedTo = toStatus
	if err := s.updateStatusErrByID[orderID]; err != nil {
		return err
	}
	if order, ok := s.ordersByID[orderID]; ok {
		order.Status = toStatus
		s.ordersByID[orderID] = order
	}
	return nil
}

func (s *stubStatusOrderRepo) ListOrdersByStatus(context.Context, int64, string) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubStatusOrderRepo) FindExpiredOrders(context.Context, int) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubStatusOrderRepo) BatchUpdateStatus(_ context.Context, orderIDs []int64, fromStatus, toStatus domain.OrderStatus) error {
	s.batchUpdatedIDs = append([]int64(nil), orderIDs...)
	s.batchUpdatedFrom = fromStatus
	s.batchUpdatedTo = toStatus
	if s.batchUpdateErr != nil {
		return s.batchUpdateErr
	}
	for _, id := range orderIDs {
		if err, ok := s.updateStatusErrByID[id]; ok && err != nil {
			return err
		}
	}
	for _, id := range orderIDs {
		if order, ok := s.ordersByID[id]; ok {
			order.Status = toStatus
			s.ordersByID[id] = order
		}
	}
	return nil
}

func (s *stubStatusOrderRepo) ListByUserID(context.Context, int64, int64, int) ([]*domain.Order, int64, error) {
	return nil, 0, nil
}

type stubOutboxRepo struct {
	addID            int64
	batchAddIDs      []int64
	markSentIDs      []int64
	batchMarkSentIDs []int64
	markFailedIDs    []int64
	retryIDs         []int64
	mu               sync.Mutex
}

func (s *stubOutboxRepo) Add(context.Context, string, any) (int64, error) {
	return s.addID, nil
}

func (s *stubOutboxRepo) BatchAdd(context.Context, string, []any) ([]int64, error) {
	return append([]int64(nil), s.batchAddIDs...), nil
}

func (s *stubOutboxRepo) ListPending(context.Context, int, int) ([]domain.OutboxEvent, error) {
	return nil, nil
}

func (s *stubOutboxRepo) MarkSent(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markSentIDs = append(s.markSentIDs, id)
	return nil
}

func (s *stubOutboxRepo) BatchMarkSent(_ context.Context, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchMarkSentIDs = append(s.batchMarkSentIDs, ids...)
	return nil
}

func (s *stubOutboxRepo) MarkFailed(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markFailedIDs = append(s.markFailedIDs, id)
	return nil
}

func (s *stubOutboxRepo) IncreaseRetry(_ context.Context, id int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retryIDs = append(s.retryIDs, id)
	return len(s.retryIDs), nil
}

type stubTxManager struct{}

func (stubTxManager) Tx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type stubSyncProducer struct {
	sendMessageErr  error
	sendMessagesErr error
}

func (s *stubSyncProducer) SendMessage(*sarama.ProducerMessage) (int32, int64, error) {
	return 0, 0, s.sendMessageErr
}

func (s *stubSyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	if s.sendMessagesErr != nil {
		return s.sendMessagesErr
	}
	return nil
}

func (s *stubSyncProducer) Close() error { return nil }

func (s *stubSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }

func (s *stubSyncProducer) IsTransactional() bool { return false }

func (s *stubSyncProducer) BeginTxn() error { return nil }

func (s *stubSyncProducer) CommitTxn() error { return nil }

func (s *stubSyncProducer) AbortTxn() error { return nil }

func (s *stubSyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}

func (s *stubSyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}
