package mq

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	orderdomain "github.com/XDWow/DouyinMall/backend/internal/order/domain"
	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/require"
)

func TestEventProcessorProcessSuccess(t *testing.T) {
	activityRepo := newProcessorActivityRepo()
	requestRepo := newProcessorRequestRepo(activityRepo)
	cache := newProcessorCache()
	cache.stock[1] = 1
	activityRepo.stockByActivity[1] = 1

	processor := NewEventProcessor(&processorOrderClient{}, requestRepo, activityRepo, cache, logger.NewNopLogger())
	evt := seckilldomain.Event{
		RequestNo:    "10001",
		ActivityID:   1,
		UserID:       2,
		ProductID:    3,
		SKUID:        4,
		SeckillPrice: 99,
	}

	err := processor.Process(context.Background(), evt)

	require.NoError(t, err)
	req, err := requestRepo.FindByRequestNo(context.Background(), evt.RequestNo)
	require.NoError(t, err)
	require.Equal(t, seckilldomain.RequestStatusSuccess, req.Status)
	require.EqualValues(t, 10001, req.OrderID)
	require.EqualValues(t, 0, activityRepo.stockByActivity[evt.ActivityID])
	require.True(t, activityRepo.hasQualification(evt.ActivityID, evt.UserID))
	result, err := cache.GetResult(context.Background(), evt.RequestNo)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, seckilldomain.RequestStatusSuccess, result.Status)
	require.EqualValues(t, 10001, result.OrderID)
}

func TestEventProcessorDeadLetterRollsBackOrderCreating(t *testing.T) {
	activityRepo := newProcessorActivityRepo()
	requestRepo := newProcessorRequestRepo(activityRepo)
	cache := newProcessorCache()
	cache.stock[1] = 0
	cache.userMarkers[processorActUserKey(1, 2)] = "10002"
	requestRepo.byReq["10002"] = &seckilldomain.Request{
		ID:         1,
		RequestNo:  "10002",
		ActivityID: 1,
		UserID:     2,
		Status:     seckilldomain.RequestStatusOrderCreating,
	}
	activityRepo.stockByActivity[1] = 0
	activityRepo.qualifications[processorActUserKey(1, 2)] = "10002"

	processor := NewEventProcessor(&processorOrderClient{}, requestRepo, activityRepo, cache, logger.NewNopLogger())
	err := processor.ProcessDeadLetter(context.Background(), seckilldomain.DeadLetterEvent{
		Event: seckilldomain.Event{
			RequestNo:  "10002",
			ActivityID: 1,
			UserID:     2,
		},
		Reason: seckilldomain.FailReasonCreateOrderFail,
	})

	require.NoError(t, err)
	req, err := requestRepo.FindByRequestNo(context.Background(), "10002")
	require.NoError(t, err)
	require.Equal(t, seckilldomain.RequestStatusFailed, req.Status)
	require.Equal(t, seckilldomain.FailReasonCreateOrderFail, req.FailReason)
	require.EqualValues(t, 1, activityRepo.stockByActivity[1])
	require.False(t, activityRepo.hasQualification(1, 2))
	require.EqualValues(t, 1, cache.stock[1])
	_, ok := cache.userMarkers[processorActUserKey(1, 2)]
	require.False(t, ok)
	result, err := cache.GetResult(context.Background(), "10002")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, seckilldomain.RequestStatusFailed, result.Status)
}

func TestEventProcessorDuplicateCreateOrderErrorDoesNotRollback(t *testing.T) {
	activityRepo := newProcessorActivityRepo()
	requestRepo := newProcessorRequestRepo(activityRepo)
	cache := newProcessorCache()
	cache.stock[1] = 0
	cache.userMarkers[processorActUserKey(1, 2)] = "10003"
	requestRepo.byReq["10003"] = &seckilldomain.Request{
		ID:         1,
		RequestNo:  "10003",
		ActivityID: 1,
		UserID:     2,
		Status:     seckilldomain.RequestStatusOrderCreating,
	}
	activityRepo.stockByActivity[1] = 0
	activityRepo.qualifications[processorActUserKey(1, 2)] = "10003"

	processor := NewEventProcessor(&processorOrderClient{
		createErr: errors.New("Error 1062: Duplicate entry '10003' for key 'PRIMARY'"),
	}, requestRepo, activityRepo, cache, logger.NewNopLogger())

	err := processor.Process(context.Background(), seckilldomain.Event{
		RequestNo:  "10003",
		ActivityID: 1,
		UserID:     2,
	})

	require.Error(t, err)
	req, findErr := requestRepo.FindByRequestNo(context.Background(), "10003")
	require.NoError(t, findErr)
	require.Equal(t, seckilldomain.RequestStatusOrderCreating, req.Status)
	require.EqualValues(t, 0, activityRepo.stockByActivity[1])
	require.True(t, activityRepo.hasQualification(1, 2))
	require.EqualValues(t, 0, cache.stock[1])
}

func TestEventProcessorDeadLetterLookupOrderNetworkErrorDoesNotRollback(t *testing.T) {
	activityRepo := newProcessorActivityRepo()
	requestRepo := newProcessorRequestRepo(activityRepo)
	cache := newProcessorCache()
	cache.stock[1] = 0
	cache.userMarkers[processorActUserKey(1, 2)] = "10004"
	requestRepo.byReq["10004"] = &seckilldomain.Request{
		ID:         1,
		RequestNo:  "10004",
		ActivityID: 1,
		UserID:     2,
		Status:     seckilldomain.RequestStatusOrderCreating,
	}
	activityRepo.stockByActivity[1] = 0
	activityRepo.qualifications[processorActUserKey(1, 2)] = "10004"

	processor := NewEventProcessor(&processorOrderClient{
		getErr: errors.New("network timeout"),
	}, requestRepo, activityRepo, cache, logger.NewNopLogger())

	err := processor.ProcessDeadLetter(context.Background(), seckilldomain.DeadLetterEvent{
		Event: seckilldomain.Event{
			RequestNo:  "10004",
			ActivityID: 1,
			UserID:     2,
		},
	})

	require.Error(t, err)
	req, findErr := requestRepo.FindByRequestNo(context.Background(), "10004")
	require.NoError(t, findErr)
	require.Equal(t, seckilldomain.RequestStatusOrderCreating, req.Status)
	require.EqualValues(t, 0, activityRepo.stockByActivity[1])
	require.True(t, activityRepo.hasQualification(1, 2))
}

func TestEventProcessorDeadLetterOrderNotFoundRollsBack(t *testing.T) {
	activityRepo := newProcessorActivityRepo()
	requestRepo := newProcessorRequestRepo(activityRepo)
	cache := newProcessorCache()
	cache.stock[1] = 0
	cache.userMarkers[processorActUserKey(1, 2)] = "10005"
	requestRepo.byReq["10005"] = &seckilldomain.Request{
		ID:         1,
		RequestNo:  "10005",
		ActivityID: 1,
		UserID:     2,
		Status:     seckilldomain.RequestStatusOrderCreating,
	}
	activityRepo.stockByActivity[1] = 0
	activityRepo.qualifications[processorActUserKey(1, 2)] = "10005"

	processor := NewEventProcessor(&processorOrderClient{
		getErr: kerrors.NewBizStatusError(orderdomain.BizStatusGetOrderNotFound, "record not found"),
	}, requestRepo, activityRepo, cache, logger.NewNopLogger())

	err := processor.ProcessDeadLetter(context.Background(), seckilldomain.DeadLetterEvent{
		Event: seckilldomain.Event{
			RequestNo:  "10005",
			ActivityID: 1,
			UserID:     2,
		},
		Reason: seckilldomain.FailReasonCreateOrderFail,
	})

	require.NoError(t, err)
	req, findErr := requestRepo.FindByRequestNo(context.Background(), "10005")
	require.NoError(t, findErr)
	require.Equal(t, seckilldomain.RequestStatusFailed, req.Status)
	require.EqualValues(t, 1, activityRepo.stockByActivity[1])
	require.False(t, activityRepo.hasQualification(1, 2))
}

type processorActivityRepo struct {
	mu              sync.Mutex
	stockByActivity map[int64]int32
	qualifications  map[string]string
}

func newProcessorActivityRepo() *processorActivityRepo {
	return &processorActivityRepo{
		stockByActivity: make(map[int64]int32),
		qualifications:  make(map[string]string),
	}
}

func (r *processorActivityRepo) Create(context.Context, *seckilldomain.Activity) error { return nil }

func (r *processorActivityRepo) FindByID(context.Context, int64) (*seckilldomain.Activity, error) {
	return nil, seckilldomain.ErrActivityNotFound
}

func (r *processorActivityRepo) UpdateStatus(context.Context, int64, string) error { return nil }

func (r *processorActivityRepo) hasQualification(activityID, userID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.qualifications[processorActUserKey(activityID, userID)]
	return ok
}

type processorRequestRepo struct {
	mu           sync.Mutex
	next         int64
	byReq        map[string]*seckilldomain.Request
	activityRepo *processorActivityRepo
}

func newProcessorRequestRepo(activityRepo *processorActivityRepo) *processorRequestRepo {
	return &processorRequestRepo{
		byReq:        make(map[string]*seckilldomain.Request),
		activityRepo: activityRepo,
	}
}

func (r *processorRequestRepo) Create(_ context.Context, request *seckilldomain.Request) error {
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

func (r *processorRequestRepo) FindByRequestNo(_ context.Context, requestNo string) (*seckilldomain.Request, error) {
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

func (r *processorRequestRepo) FindByActivityUser(_ context.Context, activityID, userID int64) (*seckilldomain.Request, error) {
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

func (r *processorRequestRepo) AdvanceProcessing(_ context.Context, evt seckilldomain.Event) (*seckilldomain.Request, error) {
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

	if r.activityRepo.stockByActivity[evt.ActivityID] < 1 {
		req.Status = seckilldomain.RequestStatusFailed
		req.FailReason = seckilldomain.FailReasonOutOfStock
		copied := *req
		return &copied, nil
	}

	key := processorActUserKey(evt.ActivityID, evt.UserID)
	if _, exists := r.activityRepo.qualifications[key]; exists {
		req.Status = seckilldomain.RequestStatusFailed
		req.FailReason = seckilldomain.FailReasonUserAlreadySucceeded
		copied := *req
		return &copied, nil
	}

	r.activityRepo.stockByActivity[evt.ActivityID]--
	r.activityRepo.qualifications[key] = evt.RequestNo

	req.Status = seckilldomain.RequestStatusOrderCreating
	req.FailReason = ""
	copied := *req
	return &copied, nil
}

func (r *processorRequestRepo) CompleteOrderCreating(_ context.Context, evt seckilldomain.Event) (*seckilldomain.Request, error) {
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

func (r *processorRequestRepo) RollbackOrderCreating(_ context.Context, evt seckilldomain.Event, failReason string) (*seckilldomain.Request, error) {
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
	r.activityRepo.stockByActivity[evt.ActivityID]++
	delete(r.activityRepo.qualifications, processorActUserKey(evt.ActivityID, evt.UserID))

	req.Status = seckilldomain.RequestStatusFailed
	req.FailReason = failReason
	copied := *req
	return &copied, nil
}

func (r *processorRequestRepo) CloseByOrderResult(_ context.Context, requestNo string, failReason string) (*seckilldomain.Request, bool, error) {
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
		r.activityRepo.stockByActivity[req.ActivityID]++
		delete(r.activityRepo.qualifications, processorActUserKey(req.ActivityID, req.UserID))
	}

	req.Status = seckilldomain.RequestStatusFailed
	req.FailReason = failReason
	copied := *req
	return &copied, true, nil
}

func (r *processorRequestRepo) MarkFail(_ context.Context, requestNo string, failReason string) error {
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

type processorCache struct {
	mu          sync.Mutex
	stock       map[int64]int32
	userMarkers map[string]string
	results     map[string]seckilldomain.Result
}

func newProcessorCache() *processorCache {
	return &processorCache{
		stock:       make(map[int64]int32),
		userMarkers: make(map[string]string),
		results:     make(map[string]seckilldomain.Result),
	}
}

func (c *processorCache) SetActivity(context.Context, *seckilldomain.Activity) error { return nil }

func (c *processorCache) GetActivity(context.Context, int64) (*seckilldomain.Activity, error) {
	return nil, nil
}

func (c *processorCache) SetActivityStock(context.Context, int64, int32) error { return nil }

func (c *processorCache) AtomicReserve(context.Context, int64, int64, string, int64) (int64, error) {
	return 0, nil
}

func (c *processorCache) Compensate(_ context.Context, activityID, userID int64, requestNo string, result seckilldomain.Result) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.results[requestNo]; ok && existing.Status == seckilldomain.RequestStatusFailed {
		return nil
	}
	key := processorActUserKey(activityID, userID)
	if c.userMarkers[key] == requestNo {
		c.stock[activityID]++
		delete(c.userMarkers, key)
	}
	c.results[requestNo] = result
	return nil
}

func (c *processorCache) SetResult(_ context.Context, result seckilldomain.Result) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[result.RequestNo] = result
	return nil
}

func (c *processorCache) GetResult(_ context.Context, requestNo string) (*seckilldomain.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.results[requestNo]
	if !ok {
		return nil, nil
	}
	copied := result
	return &copied, nil
}

func (c *processorCache) ResolveTransaction(context.Context, int64, int64, string) (seckilldomain.TransactionResolution, error) {
	return seckilldomain.TransactionResolutionUnknown, nil
}

type processorOrderClient struct {
	byID      map[int64]*orderv1.Order
	createErr error
	getErr    error
}

func (c *processorOrderClient) CreateOrder(_ context.Context, req *orderv1.CreateOrderReq, _ ...callopt.Option) (*orderv1.CreateOrderResp, error) {
	if c.createErr != nil {
		return nil, c.createErr
	}
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

func (c *processorOrderClient) ChangeOrderStatus(context.Context, *orderv1.ChangeOrderStatusReq, ...callopt.Option) (*orderv1.ChangeOrderStatusResp, error) {
	return nil, errors.New("not implemented")
}

func (c *processorOrderClient) GetOrder(_ context.Context, req *orderv1.GetOrderReq, _ ...callopt.Option) (*orderv1.GetOrderResp, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	if c.byID == nil {
		return &orderv1.GetOrderResp{}, nil
	}
	order, ok := c.byID[req.GetOrderId()]
	if !ok {
		return &orderv1.GetOrderResp{}, nil
	}
	return &orderv1.GetOrderResp{Order: order}, nil
}

func (c *processorOrderClient) ListOrder(context.Context, *orderv1.ListOrderReq, ...callopt.Option) (*orderv1.ListOrderResp, error) {
	return nil, errors.New("not implemented")
}

var _ orderservice.Client = (*processorOrderClient)(nil)

func processorActUserKey(activityID, userID int64) string {
	return strconv.FormatInt(activityID, 10) + ":" + strconv.FormatInt(userID, 10)
}
