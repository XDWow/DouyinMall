package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/internal/order/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/stretchr/testify/require"
)

func TestDispatchOrderTimeoutJobSkipsNonCreatedOrdersBeforeConfirmingPayment(t *testing.T) {
	delayQueue := &stubDelayQueue{
		dueIDs: []int64{1001},
	}
	paymentCli := &stubTimeoutPaymentClient{}
	orderRepo := &stubTimeoutOrderRepo{
		orderByID: map[int64]domain.Order{
			1001: {ID: 1001, UserID: 2001, Status: domain.OrderStatusPaid},
		},
	}
	outboxRepo := &stubTimeoutOutboxRepo{}
	producer := mq.NewSaramaProducer(stubTimeoutSyncProducer{})
	batchCancelUC := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	changeUC := usecase.NewChangeOrderStatusUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	job := NewDispatchOrderTimeoutJob(delayQueue, paymentCli, orderRepo, batchCancelUC, changeUC, logger.NewNopLogger())

	err := job.Run()

	require.NoError(t, err)
	require.Equal(t, 0, paymentCli.confirmCalls)
	require.Equal(t, 1, orderRepo.findCalls)
	require.Empty(t, delayQueue.dueIDs)
}

func TestDispatchOrderTimeoutJobLoadsCreatedOrderBeforeConfirmingPayment(t *testing.T) {
	delayQueue := &stubDelayQueue{
		dueIDs: []int64{1002},
	}
	paymentCli := &stubTimeoutPaymentClient{
		statusByTradeNo: map[string]paymentv1.PaymentStatus{
			"1002": paymentv1.PaymentStatus_PaymentStatusInit,
		},
	}
	orderRepo := &stubTimeoutOrderRepo{
		orderByID: map[int64]domain.Order{
			1002: {ID: 1002, UserID: 2002, Status: domain.OrderStatusCreated},
		},
		confirmCalls: &paymentCli.confirmCalls,
	}
	outboxRepo := &stubTimeoutOutboxRepo{batchAddIDs: []int64{1}}
	producer := mq.NewSaramaProducer(stubTimeoutSyncProducer{})
	batchCancelUC := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	changeUC := usecase.NewChangeOrderStatusUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	job := NewDispatchOrderTimeoutJob(delayQueue, paymentCli, orderRepo, batchCancelUC, changeUC, logger.NewNopLogger())

	err := job.Run()

	require.NoError(t, err)
	require.Equal(t, 1, paymentCli.confirmCalls)
	require.Equal(t, 1, orderRepo.findCalls)
	require.Equal(t, 0, orderRepo.findCallConfirmSnapshot)
	require.Equal(t, []int64{1002}, orderRepo.batchUpdatedIDs)
	require.False(t, orderRepo.findByIDsLocked)
	require.Empty(t, delayQueue.dueIDs)
}

func TestDispatchOrderTimeoutJobBatchCancelsAllDueCreatedOrders(t *testing.T) {
	delayQueue := &stubDelayQueue{
		dueIDs: []int64{1003, 1004},
	}
	paymentCli := &stubTimeoutPaymentClient{
		statusByTradeNo: map[string]paymentv1.PaymentStatus{
			"1003": paymentv1.PaymentStatus_PaymentStatusInit,
			"1004": paymentv1.PaymentStatus_PaymentStatusInit,
		},
	}
	orderRepo := &stubTimeoutOrderRepo{
		orderByID: map[int64]domain.Order{
			1003: {ID: 1003, UserID: 2003, Status: domain.OrderStatusCreated},
			1004: {ID: 1004, UserID: 2004, Status: domain.OrderStatusCreated},
		},
	}
	outboxRepo := &stubTimeoutOutboxRepo{batchAddIDs: []int64{1, 2}}
	producer := mq.NewSaramaProducer(stubTimeoutSyncProducer{})
	batchCancelUC := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	changeUC := usecase.NewChangeOrderStatusUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	job := NewDispatchOrderTimeoutJob(delayQueue, paymentCli, orderRepo, batchCancelUC, changeUC, logger.NewNopLogger())

	err := job.Run()

	require.NoError(t, err)
	require.Equal(t, []int64{1003, 1004}, orderRepo.batchUpdatedIDs)
	require.False(t, orderRepo.findByIDsLocked)
}

func TestDispatchOrderTimeoutJobSyncsPaidOrderBeforeSkippingCancel(t *testing.T) {
	delayQueue := &stubDelayQueue{
		dueIDs: []int64{1005},
	}
	paymentCli := &stubTimeoutPaymentClient{
		statusByTradeNo: map[string]paymentv1.PaymentStatus{
			"1005": paymentv1.PaymentStatus_PaymentStatusSuccess,
		},
	}
	outboxRepo := &stubTimeoutOutboxRepo{addID: 9}
	orderRepo := &stubTimeoutOrderRepo{
		orderByID: map[int64]domain.Order{
			1005: {ID: 1005, UserID: 2005, Status: domain.OrderStatusCreated, OrderKind: domain.OrderKindCart, OrderItems: []domain.OrderItem{{ProductID: 88, SKUID: 99}}},
		},
	}
	producer := mq.NewSaramaProducer(stubTimeoutSyncProducer{})
	batchCancelUC := usecase.NewBatchCancelOrderUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	changeUC := usecase.NewChangeOrderStatusUseCase(orderRepo, outboxRepo, producer, stubTimeoutTxManager{}, logger.NewNopLogger())
	job := NewDispatchOrderTimeoutJob(delayQueue, paymentCli, orderRepo, batchCancelUC, changeUC, logger.NewNopLogger())

	err := job.Run()

	require.NoError(t, err)
	require.Equal(t, int64(1005), orderRepo.updatedOrderID)
	require.Equal(t, domain.OrderStatusCreated, orderRepo.updatedFrom)
	require.Equal(t, domain.OrderStatusPaid, orderRepo.updatedTo)
	require.Empty(t, orderRepo.batchUpdatedIDs)
	require.Equal(t, domain.EventTypeOrderStatusChanged, outboxRepo.addEventType)
	paidEvent := outboxRepo.addPayload.(domain.OrderStatusUpdateEvent)
	require.Equal(t, int64(1005), paidEvent.OrderID)
	require.Equal(t, domain.OrderStatusPaid, paidEvent.Status)
	require.Equal(t, int64(2005), paidEvent.UserID)
	require.Equal(t, []int64{88}, paidEvent.ProductIDs)
	require.Equal(t, []int64{99}, paidEvent.SKUIDs)
}

type stubTimeoutPaymentClient struct {
	statusByTradeNo map[string]paymentv1.PaymentStatus
	errByTradeNo    map[string]error
	confirmCalls    int
}

func (s *stubTimeoutPaymentClient) NativePrepay(context.Context, *paymentv1.NativePrePayRequest, ...callopt.Option) (*paymentv1.NativePrePayResponse, error) {
	return nil, nil
}

func (s *stubTimeoutPaymentClient) GetPayment(context.Context, *paymentv1.GetPaymentRequest, ...callopt.Option) (*paymentv1.GetPaymentResponse, error) {
	return nil, nil
}

func (s *stubTimeoutPaymentClient) ConfirmPayment(_ context.Context, req *paymentv1.ConfirmPaymentRequest, _ ...callopt.Option) (*paymentv1.ConfirmPaymentResponse, error) {
	s.confirmCalls++
	if err := s.errByTradeNo[req.GetBizTradeNo()]; err != nil {
		return nil, err
	}
	status, ok := s.statusByTradeNo[req.GetBizTradeNo()]
	if !ok {
		return nil, errors.New("missing payment status")
	}
	return &paymentv1.ConfirmPaymentResponse{Status: status}, nil
}

type stubTimeoutTxManager struct{}

func (stubTimeoutTxManager) Tx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type stubTimeoutOutboxRepo struct {
	addID        int64
	addEventType string
	addPayload   any
	batchAddIDs  []int64
}

func (s *stubTimeoutOutboxRepo) Add(_ context.Context, eventType string, payload any) (int64, error) {
	s.addEventType = eventType
	s.addPayload = payload
	return s.addID, nil
}

func (s *stubTimeoutOutboxRepo) BatchAdd(context.Context, string, []any) ([]int64, error) {
	return append([]int64(nil), s.batchAddIDs...), nil
}

func (s *stubTimeoutOutboxRepo) ListPending(context.Context, int, int) ([]domain.OutboxEvent, error) {
	return nil, nil
}

func (s *stubTimeoutOutboxRepo) MarkSent(context.Context, int64) error {
	return nil
}

func (s *stubTimeoutOutboxRepo) BatchMarkSent(context.Context, []int64) error {
	return nil
}

func (s *stubTimeoutOutboxRepo) MarkFailed(context.Context, int64) error {
	return nil
}

func (s *stubTimeoutOutboxRepo) IncreaseRetry(context.Context, int64) (int, error) {
	return 0, nil
}

type stubTimeoutSyncProducer struct{}

func (stubTimeoutSyncProducer) SendMessage(*sarama.ProducerMessage) (int32, int64, error) {
	return 0, 0, nil
}

func (stubTimeoutSyncProducer) SendMessages([]*sarama.ProducerMessage) error {
	return nil
}

func (stubTimeoutSyncProducer) Close() error { return nil }

func (stubTimeoutSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }

func (stubTimeoutSyncProducer) IsTransactional() bool { return false }

func (stubTimeoutSyncProducer) BeginTxn() error { return nil }

func (stubTimeoutSyncProducer) CommitTxn() error { return nil }

func (stubTimeoutSyncProducer) AbortTxn() error { return nil }

func (stubTimeoutSyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}

func (stubTimeoutSyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}

type stubTimeoutOrderRepo struct {
	orderByID               map[int64]domain.Order
	findCalls               int
	findByIDsLocked         bool
	findCallConfirmSnapshot int
	confirmCalls            *int
	updatedOrderID          int64
	updatedFrom             domain.OrderStatus
	updatedTo               domain.OrderStatus
	batchUpdatedIDs         []int64
	batchUpdatedFrom        domain.OrderStatus
	batchUpdatedTo          domain.OrderStatus
}

func (s *stubTimeoutOrderRepo) Save(context.Context, *domain.Order) error {
	return nil
}

func (s *stubTimeoutOrderRepo) FindByID(_ context.Context, orderID int64) (domain.Order, error) {
	s.findCalls++
	if s.confirmCalls != nil {
		s.findCallConfirmSnapshot = *s.confirmCalls
	}
	order, ok := s.orderByID[orderID]
	if !ok {
		return domain.Order{}, domain.ErrRecordNotFound
	}
	return order, nil
}

func (s *stubTimeoutOrderRepo) FindByIDs(_ context.Context, orderIDs []int64) ([]*domain.Order, error) {
	return s.findByIDs(orderIDs)
}

func (s *stubTimeoutOrderRepo) FindByIDsForUpdate(_ context.Context, orderIDs []int64) ([]*domain.Order, error) {
	s.findByIDsLocked = true
	return s.findByIDs(orderIDs)
}

func (s *stubTimeoutOrderRepo) findByIDs(orderIDs []int64) ([]*domain.Order, error) {
	orders := make([]*domain.Order, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order, ok := s.orderByID[orderID]
		if !ok {
			return nil, domain.ErrRecordNotFound
		}
		orderCopy := order
		orders = append(orders, &orderCopy)
	}
	return orders, nil
}

func (s *stubTimeoutOrderRepo) UpdateStatus(_ context.Context, orderID int64, fromStatus, toStatus domain.OrderStatus) error {
	s.updatedOrderID = orderID
	s.updatedFrom = fromStatus
	s.updatedTo = toStatus
	if order, ok := s.orderByID[orderID]; ok {
		order.Status = toStatus
		s.orderByID[orderID] = order
	}
	return nil
}

func (s *stubTimeoutOrderRepo) ListOrdersByStatus(context.Context, int64, string) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubTimeoutOrderRepo) FindExpiredOrders(context.Context, int) ([]*domain.Order, error) {
	return nil, nil
}

func (s *stubTimeoutOrderRepo) BatchUpdateStatus(_ context.Context, orderIDs []int64, fromStatus, toStatus domain.OrderStatus) error {
	s.batchUpdatedIDs = append([]int64(nil), orderIDs...)
	s.batchUpdatedFrom = fromStatus
	s.batchUpdatedTo = toStatus
	return nil
}

func (s *stubTimeoutOrderRepo) ListByUserID(context.Context, int64, int64, int) ([]*domain.Order, int64, error) {
	return nil, 0, nil
}

type stubDelayQueue struct {
	dueIDs      []int64
	enqueuedIDs []int64
}

func (s *stubDelayQueue) Enqueue(_ context.Context, orderID int64, _ time.Time) error {
	s.enqueuedIDs = append(s.enqueuedIDs, orderID)
	return nil
}

func (s *stubDelayQueue) DrainDue(context.Context, time.Time) ([]int64, error) {
	ids := append([]int64(nil), s.dueIDs...)
	s.dueIDs = nil
	return ids, nil
}
