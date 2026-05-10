//go:build integration
// +build integration

package mq

import (
	"context"
	"errors"
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
	"github.com/stretchr/testify/require"
)

func TestSeckillTransactionalFlowHappyPath(t *testing.T) {
	ctx := context.Background()
	activityRepo := newMemoryActivityRepo()
	requestRepo := newMemoryRequestRepo(activityRepo)
	cache := newMemoryCache()
	producer := &capturingProducer{}
	soldOut := seckilldomain.NewNopSoldOutMarker()
	idGen := sequenceIDGenerator("10001")

	createActivityUC := seckillusecase.NewCreateActivityUseCase(activityRepo, cache, soldOut)
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

	submitUC := seckillusecase.NewSubmitUseCase(activityRepo, requestRepo, cache, soldOut, producer, idGen)
	result, err := submitUC.Execute(ctx, seckillusecase.SubmitCmd{
		ActivityID: activityID,
		UserID:     2001,
	})
	require.NoError(t, err)
	require.Equal(t, seckilldomain.RequestStatusProcessing, result.Status)
	require.Len(t, producer.prepared, 1)

	processor := NewEventProcessor(&recordingOrderClient{}, requestRepo, activityRepo, cache, soldOut, logger.NewNopLogger())
	require.NoError(t, processor.Process(ctx, producer.prepared[0]))

	req, err := requestRepo.FindByRequestNo(ctx, result.RequestNo)
	require.NoError(t, err)
	require.Equal(t, seckilldomain.RequestStatusSuccess, req.Status)
	require.EqualValues(t, 10001, req.OrderID)

	cached, err := cache.GetResult(ctx, result.RequestNo)
	require.NoError(t, err)
	require.NotNil(t, cached)
	require.Equal(t, seckilldomain.RequestStatusSuccess, cached.Status)
	require.EqualValues(t, 10001, cached.OrderID)

	activity, err := activityRepo.FindByID(ctx, activityID)
	require.NoError(t, err)
	require.EqualValues(t, 0, activity.AvailableStock)
	require.True(t, activityRepo.hasQualification(activityID, 2001))
	require.True(t, cache.hasUser(activityID, 2001, result.RequestNo))

	statusConsumer := NewOrderStatusConsumer(nil, requestRepo, activityRepo, cache, soldOut, logger.NewNopLogger(), SeckillConsumerOptions{})
	require.NoError(t, statusConsumer.consume(context.Background(), OrderStatusUpdateEvent{
		OrderID:   req.OrderID,
		Status:    orderv1.OrderStatus_ORDER_STATUS_CANCELED,
		OrderKind: orderdomain.OrderKindSeckill,
	}))

	req, err = requestRepo.FindByRequestNo(ctx, result.RequestNo)
	require.NoError(t, err)
	require.Equal(t, seckilldomain.RequestStatusFailed, req.Status)
	require.Equal(t, seckilldomain.FailReasonOrderCanceled, req.FailReason)
	require.Zero(t, req.OrderID)

	cached, err = cache.GetResult(ctx, result.RequestNo)
	require.NoError(t, err)
	require.NotNil(t, cached)
	require.Equal(t, seckilldomain.RequestStatusFailed, cached.Status)
	require.Equal(t, seckilldomain.FailReasonOrderCanceled, cached.FailReason)

	activity, err = activityRepo.FindByID(ctx, activityID)
	require.NoError(t, err)
	require.EqualValues(t, 1, activity.AvailableStock)
	require.False(t, activityRepo.hasQualification(activityID, 2001))
	require.False(t, cache.hasUser(activityID, 2001, result.RequestNo))
}

func TestResolveTransactionRecoversProcessingFromUserMarker(t *testing.T) {
	cache := newMemoryCache()
	cache.userMarkers[memoryActUserKey(1, 2)] = "10002"

	resolution, err := cache.ResolveTransaction(context.Background(), 1, 2, "10002")

	require.NoError(t, err)
	require.Equal(t, seckilldomain.TransactionResolutionCommit, resolution)

	result, err := cache.GetResult(context.Background(), "10002")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, seckilldomain.RequestStatusProcessing, result.Status)
}

func TestResolveTransactionRollsBackFailedRequest(t *testing.T) {
	cache := newMemoryCache()
	cache.results["10003"] = seckilldomain.Result{
		RequestNo:  "10003",
		Status:     seckilldomain.RequestStatusFailed,
		FailReason: seckilldomain.FailReasonOutOfStock,
	}

	resolution, err := cache.ResolveTransaction(context.Background(), 1, 2, "10003")

	require.NoError(t, err)
	require.Equal(t, seckilldomain.TransactionResolutionRollback, resolution)
}

func TestResolveTransactionCommitsWhenStateIsMissing(t *testing.T) {
	cache := newMemoryCache()

	resolution, err := cache.ResolveTransaction(context.Background(), 1, 2, "10004")

	require.NoError(t, err)
	require.Equal(t, seckilldomain.TransactionResolutionCommit, resolution)

	result, err := cache.GetResult(context.Background(), "10004")
	require.NoError(t, err)
	require.Nil(t, result)
}

type capturingProducer struct {
	mu       sync.Mutex
	prepared []seckilldomain.Event
}

func (p *capturingProducer) Submit(_ context.Context, evt seckilldomain.Event, _ int64) (*seckilldomain.Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prepared = append(p.prepared, evt)
	return &seckilldomain.Result{
		RequestNo: evt.RequestNo,
		Status:    seckilldomain.RequestStatusProcessing,
	}, nil
}

type sequenceIDGenerator string

func (g sequenceIDGenerator) GenerateID() string {
	return string(g)
}

type recordingOrderClient struct {
	mu      sync.Mutex
	created []*orderv1.CreateOrderReq
	byID    map[int64]*orderv1.Order
}

func (c *recordingOrderClient) CreateOrder(_ context.Context, req *orderv1.CreateOrderReq, _ ...callopt.Option) (*orderv1.CreateOrderResp, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.created = append(c.created, req)
	if c.byID == nil {
		c.byID = make(map[int64]*orderv1.Order)
	}
	if _, ok := c.byID[req.GetOrderId()]; ok {
		return &orderv1.CreateOrderResp{OrderId: req.GetOrderId()}, nil
	}
	c.byID[req.GetOrderId()] = &orderv1.Order{
		OrderId:    req.GetOrderId(),
		UserId:     req.GetUserId(),
		OrderKind:  orderdomain.OrderKindSeckill,
		ActivityId: req.GetActivityId(),
	}
	return &orderv1.CreateOrderResp{OrderId: req.GetOrderId()}, nil
}

func (c *recordingOrderClient) ChangeOrderStatus(context.Context, *orderv1.ChangeOrderStatusReq, ...callopt.Option) (*orderv1.ChangeOrderStatusResp, error) {
	return nil, errors.New("not implemented")
}

func (c *recordingOrderClient) GetOrder(_ context.Context, req *orderv1.GetOrderReq, _ ...callopt.Option) (*orderv1.GetOrderResp, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		return &orderv1.GetOrderResp{}, nil
	}
	order, ok := c.byID[req.GetOrderId()]
	if !ok {
		return &orderv1.GetOrderResp{}, nil
	}
	return &orderv1.GetOrderResp{Order: order}, nil
}

func (c *recordingOrderClient) ListOrder(context.Context, *orderv1.ListOrderReq, ...callopt.Option) (*orderv1.ListOrderResp, error) {
	return nil, errors.New("not implemented")
}

var _ orderservice.Client = (*recordingOrderClient)(nil)

type memoryActivityRepo struct {
	mu             sync.Mutex
	nextID         int64
	activities     map[int64]*seckilldomain.Activity
	qualifications map[string]string
}

func newMemoryActivityRepo() *memoryActivityRepo {
	return &memoryActivityRepo{
		activities:     make(map[int64]*seckilldomain.Activity),
		qualifications: make(map[string]string),
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

func (r *memoryActivityRepo) hasQualification(activityID, userID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.qualifications[memoryActUserKey(activityID, userID)]
	return ok
}

type memoryRequestRepo struct {
	mu           sync.Mutex
	next         int64
	byReq        map[string]*seckilldomain.Request
	activityRepo *memoryActivityRepo
}

func newMemoryRequestRepo(activityRepo *memoryActivityRepo) *memoryRequestRepo {
	return &memoryRequestRepo{
		byReq:        make(map[string]*seckilldomain.Request),
		activityRepo: activityRepo,
	}
}

func (r *memoryRequestRepo) Create(_ context.Context, request *seckilldomain.Request) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byReq[request.RequestNo]; ok {
		return seckilldomain.ErrDuplicateSeckill
	}
	r.next++
	copied := *request
	copied.ID = r.next
	r.byReq[request.RequestNo] = &copied
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
	if copied.Status == seckilldomain.RequestStatusSuccess {
		if orderID, ok := seckilldomain.OrderIDFromRequestNo(copied.RequestNo); ok {
			copied.OrderID = orderID
		}
	}
	return &copied, nil
}

func (r *memoryRequestRepo) FindByActivityUser(_ context.Context, activityID, userID int64) (*seckilldomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, req := range r.byReq {
		if req.ActivityID == activityID && req.UserID == userID {
			copied := *req
			return &copied, nil
		}
	}
	return nil, nil
}

func (r *memoryRequestRepo) AdvanceProcessing(_ context.Context, evt seckilldomain.Event) (*seckilldomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	req, ok := r.byReq[evt.RequestNo]
	if !ok {
		return nil, seckilldomain.ErrRequestNotFound
	}
	if req.Status != seckilldomain.RequestStatusProcessing {
		copied := *req
		return &copied, nil
	}

	r.activityRepo.mu.Lock()
	defer r.activityRepo.mu.Unlock()

	activity, exists := r.activityRepo.activities[evt.ActivityID]
	if !exists {
		return nil, seckilldomain.ErrActivityNotFound
	}
	if activity.AvailableStock < 1 {
		req.Status = seckilldomain.RequestStatusFailed
		req.FailReason = seckilldomain.FailReasonOutOfStock
		copied := *req
		return &copied, nil
	}

	key := memoryActUserKey(evt.ActivityID, evt.UserID)
	if _, exists = r.activityRepo.qualifications[key]; exists {
		req.Status = seckilldomain.RequestStatusFailed
		req.FailReason = seckilldomain.FailReasonUserAlreadySucceeded
		copied := *req
		return &copied, nil
	}

	activity.AvailableStock--
	r.activityRepo.qualifications[key] = evt.RequestNo

	req.Status = seckilldomain.RequestStatusOrderCreating
	req.FailReason = ""
	copied := *req
	return &copied, nil
}

func (r *memoryRequestRepo) CompleteOrderCreating(_ context.Context, evt seckilldomain.Event) (*seckilldomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	req, ok := r.byReq[evt.RequestNo]
	if !ok {
		return nil, seckilldomain.ErrRequestNotFound
	}
	if req.Status == seckilldomain.RequestStatusSuccess {
		copied := *req
		if orderID, ok := seckilldomain.OrderIDFromRequestNo(copied.RequestNo); ok {
			copied.OrderID = orderID
		}
		return &copied, nil
	}
	if req.Status != seckilldomain.RequestStatusOrderCreating {
		return nil, errors.New("request is not ORDER_CREATING")
	}

	req.Status = seckilldomain.RequestStatusSuccess
	req.FailReason = ""
	copied := *req
	if orderID, ok := seckilldomain.OrderIDFromRequestNo(copied.RequestNo); ok {
		copied.OrderID = orderID
	}
	return &copied, nil
}

func (r *memoryRequestRepo) RollbackOrderCreating(_ context.Context, evt seckilldomain.Event, failReason string) (*seckilldomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	req, ok := r.byReq[evt.RequestNo]
	if !ok {
		return nil, seckilldomain.ErrRequestNotFound
	}
	if req.Status == seckilldomain.RequestStatusFailed {
		copied := *req
		return &copied, nil
	}
	if req.Status != seckilldomain.RequestStatusOrderCreating {
		return nil, errors.New("request is not ORDER_CREATING")
	}

	r.activityRepo.mu.Lock()
	defer r.activityRepo.mu.Unlock()
	r.activityRepo.activities[evt.ActivityID].AvailableStock++
	delete(r.activityRepo.qualifications, memoryActUserKey(evt.ActivityID, evt.UserID))

	req.Status = seckilldomain.RequestStatusFailed
	req.FailReason = failReason
	copied := *req
	return &copied, nil
}

func (r *memoryRequestRepo) CloseByOrderResult(_ context.Context, requestNo string, failReason string) (*seckilldomain.Request, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	req, ok := r.byReq[requestNo]
	if !ok {
		return nil, false, seckilldomain.ErrRequestNotFound
	}
	if req.Status == seckilldomain.RequestStatusFailed {
		copied := *req
		return &copied, false, nil
	}

	r.activityRepo.mu.Lock()
	defer r.activityRepo.mu.Unlock()
	if req.Status == seckilldomain.RequestStatusOrderCreating || req.Status == seckilldomain.RequestStatusSuccess {
		r.activityRepo.activities[req.ActivityID].AvailableStock++
		delete(r.activityRepo.qualifications, memoryActUserKey(req.ActivityID, req.UserID))
	}

	req.Status = seckilldomain.RequestStatusFailed
	req.FailReason = failReason
	copied := *req
	return &copied, true, nil
}

func (r *memoryRequestRepo) MarkFail(_ context.Context, requestNo string, failReason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.byReq[requestNo]
	if !ok {
		return seckilldomain.ErrRequestNotFound
	}
	req.Status = seckilldomain.RequestStatusFailed
	req.FailReason = failReason
	return nil
}

type memoryCache struct {
	mu          sync.Mutex
	activities  map[int64]*seckilldomain.Activity
	stock       map[int64]int32
	userMarkers map[string]string
	results     map[string]seckilldomain.Result
}

func newMemoryCache() *memoryCache {
	return &memoryCache{
		activities:  make(map[int64]*seckilldomain.Activity),
		stock:       make(map[int64]int32),
		userMarkers: make(map[string]string),
		results:     make(map[string]seckilldomain.Result),
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

	key := memoryActUserKey(activityID, userID)
	if _, ok := c.userMarkers[key]; ok {
		return 2, nil
	}
	if c.stock[activityID] <= 0 {
		return 1, nil
	}

	c.stock[activityID]--
	c.userMarkers[key] = requestNo
	c.results[requestNo] = seckilldomain.Result{
		RequestNo: requestNo,
		Status:    seckilldomain.RequestStatusProcessing,
	}
	return 0, nil
}

func (c *memoryCache) Compensate(_ context.Context, activityID, userID int64, requestNo string, result seckilldomain.Result) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.results[requestNo]; ok && existing.Status == seckilldomain.RequestStatusFailed {
		return nil
	}
	key := memoryActUserKey(activityID, userID)
	if c.userMarkers[key] == requestNo {
		c.stock[activityID]++
		delete(c.userMarkers, key)
	}
	c.results[requestNo] = result
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

func (c *memoryCache) ResolveTransaction(_ context.Context, activityID, userID int64, requestNo string) (seckilldomain.TransactionResolution, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if result, ok := c.results[requestNo]; ok {
		if result.Status == seckilldomain.RequestStatusFailed {
			return seckilldomain.TransactionResolutionRollback, nil
		}
		return seckilldomain.TransactionResolutionCommit, nil
	}

	if c.userMarkers[memoryActUserKey(activityID, userID)] == requestNo {
		c.results[requestNo] = seckilldomain.Result{
			RequestNo: requestNo,
			Status:    seckilldomain.RequestStatusProcessing,
		}
		return seckilldomain.TransactionResolutionCommit, nil
	}
	if _, ok := c.userMarkers[memoryActUserKey(activityID, userID)]; !ok {
		return seckilldomain.TransactionResolutionCommit, nil
	}
	return seckilldomain.TransactionResolutionRollback, nil
}

func (c *memoryCache) hasUser(activityID, userID int64, requestNo string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.userMarkers[memoryActUserKey(activityID, userID)] == requestNo
}

func memoryActUserKey(activityID, userID int64) string {
	return strconv.FormatInt(activityID, 10) + ":" + strconv.FormatInt(userID, 10)
}
