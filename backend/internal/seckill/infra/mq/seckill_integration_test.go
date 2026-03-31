//go:build integration
// +build integration

package mq

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	orderdomain "github.com/XDWow/DouyinMall/backend/internal/order/domain"
	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	seckillusecase "github.com/XDWow/DouyinMall/backend/internal/seckill/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeckillHappyPath(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNopLogger()
	activityRepo := newMemoryActivityRepo()
	requestRepo := newMemoryRequestRepo()
	cache := newMemoryCache()
	producer := &capturingProducer{}
	idGen := &sequenceIDGenerator{}

	createActivityUC := seckillusecase.NewCreateActivityUseCase(activityRepo, cache)
	activityID, err := createActivityUC.Execute(ctx, seckillusecase.CreateActivityCmd{
		ActivityName: "iphone seckill",
		ProductID:    3001,
		SKUID:        4001,
		SeckillPrice: 9900,
		TotalStock:   1,
		StartTime:    time.Now().Add(-time.Minute),
		EndTime:      time.Now().Add(10 * time.Minute),
		Status:       seckilldomain.ActivityStatusOnline,
		LimitPerUser: 1,
	})
	require.NoError(t, err)

	submitUC := seckillusecase.NewSubmitUseCase(activityRepo, requestRepo, cache, producer, idGen)
	result, err := submitUC.Execute(ctx, seckillusecase.SubmitCmd{
		ActivityID: activityID,
		UserID:     2001,
	})
	require.NoError(t, err)
	require.Len(t, producer.events, 1)
	assert.Equal(t, seckilldomain.RequestStatusProcessing, result.Status)

	orderClient := &recordingOrderClient{}
	consumer := NewSeckillConsumer(nil, nil, orderClient, requestRepo, activityRepo, cache, log)
	require.NoError(t, consumer.processCreateOrderWithRetry(nil, producer.events[0]))

	req, err := requestRepo.FindByRequestNo(ctx, result.RequestNo)
	require.NoError(t, err)
	assert.Equal(t, seckilldomain.RequestStatusSuccess, req.Status)
	assert.Equal(t, int64(1), req.OrderID)

	require.Len(t, orderClient.created, 1)
	created := orderClient.created[0]
	assert.Equal(t, orderdomain.OrderKindSeckill, created.OrderKind)
	assert.Equal(t, activityID, created.ActivityId)
	require.Len(t, created.Items, 1)
	assert.Equal(t, int64(4001), created.Items[0].GetSkuId())
	assert.Equal(t, int64(9900), created.Items[0].GetConvertedPrice())

	activity, err := activityRepo.FindByID(ctx, activityID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), activity.AvailableStock)

	cached, err := cache.GetResult(ctx, result.RequestNo)
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, seckilldomain.RequestStatusSuccess, cached.Status)
	assert.Equal(t, req.OrderID, cached.OrderID)

	statusConsumer := NewOrderStatusConsumer(nil, requestRepo, activityRepo, cache, log)
	require.NoError(t, statusConsumer.consume(nil, OrderStatusUpdateEvent{
		OrderID: req.OrderID,
		Status:  orderv1.OrderStatus_ORDER_STATUS_CANCELED,
	}))

	activity, err = activityRepo.FindByID(ctx, activityID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), activity.AvailableStock)
	assert.Equal(t, int32(1), cache.stock[activityID])
}

func TestSeckillCreateOrderFailCompensates(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNopLogger()
	activityRepo := newMemoryActivityRepo()
	requestRepo := newMemoryRequestRepo()
	cache := newMemoryCache()
	producer := &capturingProducer{}

	createActivityUC := seckillusecase.NewCreateActivityUseCase(activityRepo, cache)
	activityID, err := createActivityUC.Execute(ctx, seckillusecase.CreateActivityCmd{
		ActivityName: "iphone seckill",
		ProductID:    3001,
		SKUID:        4001,
		SeckillPrice: 9900,
		TotalStock:   1,
		StartTime:    time.Now().Add(-time.Minute),
		EndTime:      time.Now().Add(10 * time.Minute),
		Status:       seckilldomain.ActivityStatusOnline,
		LimitPerUser: 1,
	})
	require.NoError(t, err)

	submitUC := seckillusecase.NewSubmitUseCase(activityRepo, requestRepo, cache, producer, &sequenceIDGenerator{})
	result, err := submitUC.Execute(ctx, seckillusecase.SubmitCmd{
		ActivityID: activityID,
		UserID:     2002,
	})
	require.NoError(t, err)
	require.Len(t, producer.events, 1)

	consumer := NewSeckillConsumer(nil, nil, &failingOrderClientStub{}, requestRepo, activityRepo, cache, log)
	require.NoError(t, consumer.processCreateOrderWithRetry(nil, producer.events[0]))

	req, err := requestRepo.FindByRequestNo(ctx, result.RequestNo)
	require.NoError(t, err)
	assert.Equal(t, seckilldomain.RequestStatusFail, req.Status)
	assert.Equal(t, seckilldomain.FailReasonCreateOrderFail, req.FailReason)

	activity, err := activityRepo.FindByID(ctx, activityID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), activity.AvailableStock)
	assert.Equal(t, int32(1), cache.stock[activityID])
	assert.False(t, cache.hasUser(activityID, 2002))

	cached, err := cache.GetResult(ctx, result.RequestNo)
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, seckilldomain.RequestStatusFail, cached.Status)
	assert.Equal(t, seckilldomain.FailReasonCreateOrderFail, cached.FailReason)
}

type capturingProducer struct {
	mu     sync.Mutex
	events []seckilldomain.Event
}

func (p *capturingProducer) Publish(_ context.Context, evt seckilldomain.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, evt)
	return nil
}

type sequenceIDGenerator struct {
	mu  sync.Mutex
	seq int64
}

func (g *sequenceIDGenerator) GenerateID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return strconv.FormatInt(g.seq, 10)
}

type recordingOrderClient struct {
	mu      sync.Mutex
	created []*orderv1.CreateOrderReq
}

func (c *recordingOrderClient) CreateOrder(_ context.Context, req *orderv1.CreateOrderReq, _ ...callopt.Option) (*orderv1.CreateOrderResp, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.created = append(c.created, req)
	return &orderv1.CreateOrderResp{OrderId: req.GetOrderId()}, nil
}

func (c *recordingOrderClient) ChangeOrderStatus(context.Context, *orderv1.ChangeOrderStatusReq, ...callopt.Option) (*orderv1.ChangeOrderStatusResp, error) {
	return nil, errors.New("not implemented")
}

func (c *recordingOrderClient) GetOrder(context.Context, *orderv1.GetOrderReq, ...callopt.Option) (*orderv1.GetOrderResp, error) {
	return nil, errors.New("not implemented")
}

func (c *recordingOrderClient) ListOrder(context.Context, *orderv1.ListOrderReq, ...callopt.Option) (*orderv1.ListOrderResp, error) {
	return nil, errors.New("not implemented")
}

var _ orderservice.Client = (*recordingOrderClient)(nil)

type failingOrderClientStub struct{}

func (f *failingOrderClientStub) CreateOrder(context.Context, *orderv1.CreateOrderReq, ...callopt.Option) (*orderv1.CreateOrderResp, error) {
	return nil, errors.New("create order failed")
}

func (f *failingOrderClientStub) ChangeOrderStatus(context.Context, *orderv1.ChangeOrderStatusReq, ...callopt.Option) (*orderv1.ChangeOrderStatusResp, error) {
	return nil, errors.New("not implemented")
}

func (f *failingOrderClientStub) GetOrder(context.Context, *orderv1.GetOrderReq, ...callopt.Option) (*orderv1.GetOrderResp, error) {
	return nil, errors.New("not implemented")
}

func (f *failingOrderClientStub) ListOrder(context.Context, *orderv1.ListOrderReq, ...callopt.Option) (*orderv1.ListOrderResp, error) {
	return nil, errors.New("not implemented")
}

var _ orderservice.Client = (*failingOrderClientStub)(nil)

type memoryActivityRepo struct {
	mu         sync.Mutex
	nextID     int64
	activities map[int64]*seckilldomain.Activity
	operations map[string]struct{}
}

func newMemoryActivityRepo() *memoryActivityRepo {
	return &memoryActivityRepo{
		activities: make(map[int64]*seckilldomain.Activity),
		operations: make(map[string]struct{}),
	}
}

func (r *memoryActivityRepo) Create(_ context.Context, activity *seckilldomain.Activity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	copied := *activity
	copied.ID = r.nextID
	r.activities[copied.ID] = &copied
	activity.ID = copied.ID
	return nil
}

func (r *memoryActivityRepo) FindByID(_ context.Context, activityID int64) (*seckilldomain.Activity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	activity, ok := r.activities[activityID]
	if !ok {
		return nil, seckilldomain.ErrActivityNotFound
	}
	copied := *activity
	return &copied, nil
}

func (r *memoryActivityRepo) UpdateStatus(_ context.Context, activityID int64, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	activity, ok := r.activities[activityID]
	if !ok {
		return seckilldomain.ErrActivityNotFound
	}
	activity.Status = status
	return nil
}

func (r *memoryActivityRepo) DecreaseStock(_ context.Context, activityID int64, requestNo string, quantity int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	op := "deduct_" + requestNo
	if _, ok := r.operations[op]; ok {
		return nil
	}
	activity, ok := r.activities[activityID]
	if !ok {
		return seckilldomain.ErrActivityNotFound
	}
	if activity.AvailableStock < quantity {
		return seckilldomain.ErrOutOfStock
	}
	activity.AvailableStock -= quantity
	r.operations[op] = struct{}{}
	return nil
}

func (r *memoryActivityRepo) IncreaseStock(_ context.Context, activityID int64, operationID string, quantity int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.operations[operationID]; ok {
		return nil
	}
	activity, ok := r.activities[activityID]
	if !ok {
		return seckilldomain.ErrActivityNotFound
	}
	activity.AvailableStock += quantity
	r.operations[operationID] = struct{}{}
	return nil
}

type memoryRequestRepo struct {
	mu        sync.Mutex
	nextID    int64
	byReq     map[string]*seckilldomain.Request
	byActUser map[string]string
}

func newMemoryRequestRepo() *memoryRequestRepo {
	return &memoryRequestRepo{
		byReq:     make(map[string]*seckilldomain.Request),
		byActUser: make(map[string]string),
	}
}

func (r *memoryRequestRepo) Create(_ context.Context, request *seckilldomain.Request) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := actUserKey(request.ActivityID, request.UserID)
	if _, ok := r.byActUser[key]; ok {
		return fmt.Errorf("duplicate request")
	}
	r.nextID++
	copied := *request
	copied.ID = r.nextID
	r.byReq[copied.RequestNo] = &copied
	r.byActUser[key] = copied.RequestNo
	request.ID = copied.ID
	return nil
}

func (r *memoryRequestRepo) FindByRequestNo(_ context.Context, requestNo string) (*seckilldomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.byReq[requestNo]
	if !ok {
		return nil, seckilldomain.ErrRequestNotFound
	}
	copied := *req
	return &copied, nil
}

func (r *memoryRequestRepo) FindByOrderID(_ context.Context, orderID int64) (*seckilldomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, req := range r.byReq {
		if req.OrderID == orderID {
			copied := *req
			return &copied, nil
		}
	}
	return nil, seckilldomain.ErrRequestNotFound
}

func (r *memoryRequestRepo) FindByActivityUser(_ context.Context, activityID, userID int64) (*seckilldomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	reqNo, ok := r.byActUser[actUserKey(activityID, userID)]
	if !ok {
		return nil, nil
	}
	copied := *r.byReq[reqNo]
	return &copied, nil
}

func (r *memoryRequestRepo) MarkSuccess(_ context.Context, requestNo string, orderID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.byReq[requestNo]
	if !ok {
		return seckilldomain.ErrRequestNotFound
	}
	req.Status = seckilldomain.RequestStatusSuccess
	req.OrderID = orderID
	req.FailReason = ""
	return nil
}

func (r *memoryRequestRepo) MarkFail(_ context.Context, requestNo string, failReason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.byReq[requestNo]
	if !ok {
		return seckilldomain.ErrRequestNotFound
	}
	req.Status = seckilldomain.RequestStatusFail
	req.FailReason = failReason
	return nil
}

type memoryCache struct {
	mu         sync.Mutex
	activities map[int64]*seckilldomain.Activity
	stock      map[int64]int32
	users      map[string]struct{}
	results    map[string]seckilldomain.Result
}

func newMemoryCache() *memoryCache {
	return &memoryCache{
		activities: make(map[int64]*seckilldomain.Activity),
		stock:      make(map[int64]int32),
		users:      make(map[string]struct{}),
		results:    make(map[string]seckilldomain.Result),
	}
}

func (c *memoryCache) SetActivity(_ context.Context, activity *seckilldomain.Activity) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	copied := *activity
	c.activities[activity.ID] = &copied
	return nil
}

func (c *memoryCache) GetActivity(_ context.Context, activityID int64) (*seckilldomain.Activity, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	activity, ok := c.activities[activityID]
	if !ok {
		return nil, nil
	}
	copied := *activity
	return &copied, nil
}

func (c *memoryCache) SetActivityStock(_ context.Context, activityID int64, stock int32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stock[activityID] = stock
	return nil
}

func (c *memoryCache) AtomicReserve(_ context.Context, activityID, userID int64, requestNo string, _ int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := actUserKey(activityID, userID)
	if _, ok := c.users[key]; ok {
		return 2, nil
	}
	if c.stock[activityID] <= 0 {
		return 1, nil
	}
	c.stock[activityID]--
	c.users[key] = struct{}{}
	c.results[requestNo] = seckilldomain.Result{
		RequestNo: requestNo,
		Status:    seckilldomain.RequestStatusProcessing,
	}
	return 0, nil
}

func (c *memoryCache) Compensate(_ context.Context, activityID, userID int64, quantity int32, removeUser bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stock[activityID] += quantity
	if removeUser {
		delete(c.users, actUserKey(activityID, userID))
	}
	return nil
}

func (c *memoryCache) IncreaseStock(_ context.Context, activityID int64, quantity int32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stock[activityID] += quantity
	return nil
}

func (c *memoryCache) SetResult(_ context.Context, result seckilldomain.Result) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[result.RequestNo] = result
	return nil
}

func (c *memoryCache) GetResult(_ context.Context, requestNo string) (*seckilldomain.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.results[requestNo]
	if !ok {
		return nil, nil
	}
	copied := result
	return &copied, nil
}

func (c *memoryCache) hasUser(activityID, userID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.users[actUserKey(activityID, userID)]
	return ok
}

func actUserKey(activityID, userID int64) string {
	return strconv.FormatInt(activityID, 10) + ":" + strconv.FormatInt(userID, 10)
}
