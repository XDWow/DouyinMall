package mq

import (
	"context"
	"errors"
	"fmt"

	orderdomain "github.com/XDWow/DouyinMall/backend/internal/order/domain"
	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

type EventProcessor struct {
	orderClient orderservice.Client
	requestRepo seckilldomain.RequestRepository
	cache       seckilldomain.Cache
	soldOut     seckilldomain.SoldOutMarker
	logger      logger.LoggerV1
}

func NewEventProcessor(orderClient orderservice.Client, requestRepo seckilldomain.RequestRepository, _ seckilldomain.ActivityRepository, cache seckilldomain.Cache, soldOut seckilldomain.SoldOutMarker, l logger.LoggerV1) *EventProcessor {
	if soldOut == nil {
		soldOut = seckilldomain.NewNopSoldOutMarker()
	}
	return &EventProcessor{
		orderClient: orderClient,
		requestRepo: requestRepo,
		cache:       cache,
		soldOut:     soldOut,
		logger:      l,
	}
}

func (p *EventProcessor) Process(ctx context.Context, evt seckilldomain.Event) error {
	req, err := p.ensureRequest(ctx, evt)
	if err != nil {
		return err
	}

	switch req.Status {
	case seckilldomain.RequestStatusProcessing:
		return p.processProcessing(ctx, evt)
	case seckilldomain.RequestStatusOrderCreating:
		return p.processOrderCreating(ctx, evt)
	case seckilldomain.RequestStatusSuccess:
		return p.cache.SetResult(ctx, seckilldomain.Result{
			RequestNo: evt.RequestNo,
			Status:    seckilldomain.RequestStatusSuccess,
			OrderID:   req.OrderID,
		})
	case seckilldomain.RequestStatusFailed:
		return p.compensateAndClear(ctx, evt.ActivityID, evt.UserID, evt.RequestNo, seckilldomain.Result{
			RequestNo:  evt.RequestNo,
			Status:     seckilldomain.RequestStatusFailed,
			FailReason: req.FailReason,
		})
	default:
		return errors.New("unknown seckill request status")
	}
}

func (p *EventProcessor) ProcessDeadLetter(ctx context.Context, dead seckilldomain.DeadLetterEvent) error {
	evt := dead.Event
	reason := deadLetterReason(dead)
	req, err := p.requestRepo.FindByRequestNo(ctx, evt.RequestNo)
	if err != nil {
		if errors.Is(err, seckilldomain.ErrRequestNotFound) {
			return p.compensateAndClear(ctx, evt.ActivityID, evt.UserID, evt.RequestNo, seckilldomain.Result{
				RequestNo:  evt.RequestNo,
				Status:     seckilldomain.RequestStatusFailed,
				FailReason: reason,
			})
		}
		return err
	}

	switch req.Status {
	case seckilldomain.RequestStatusSuccess:
		return p.cache.SetResult(ctx, seckilldomain.Result{
			RequestNo: evt.RequestNo,
			Status:    seckilldomain.RequestStatusSuccess,
			OrderID:   req.OrderID,
		})
	case seckilldomain.RequestStatusFailed:
		return p.compensateAndClear(ctx, evt.ActivityID, evt.UserID, evt.RequestNo, seckilldomain.Result{
			RequestNo:  evt.RequestNo,
			Status:     seckilldomain.RequestStatusFailed,
			FailReason: req.FailReason,
		})
	case seckilldomain.RequestStatusProcessing:
		if err = p.requestRepo.MarkFail(ctx, evt.RequestNo, reason); err != nil {
			return err
		}
		return p.compensateAndClear(ctx, evt.ActivityID, evt.UserID, evt.RequestNo, seckilldomain.Result{
			RequestNo:  evt.RequestNo,
			Status:     seckilldomain.RequestStatusFailed,
			FailReason: reason,
		})
	case seckilldomain.RequestStatusOrderCreating:
		_, ok, lookupErr := p.lookupOrder(ctx, evt)
		if lookupErr != nil {
			return lookupErr
		}
		if ok {
			req, completeErr := p.requestRepo.CompleteOrderCreating(ctx, evt)
			if completeErr != nil {
				return completeErr
			}
			return p.cache.SetResult(ctx, seckilldomain.Result{
				RequestNo: evt.RequestNo,
				Status:    seckilldomain.RequestStatusSuccess,
				OrderID:   orderIDOrRequestNo(req),
			})
		}
		return p.rollbackOrderCreating(ctx, evt, reason)
	default:
		return errors.New("unknown seckill dead-letter request status")
	}
}

func deadLetterReason(dead seckilldomain.DeadLetterEvent) string {
	if dead.Reason != "" {
		return dead.Reason
	}
	return seckilldomain.FailReasonCreateOrderFail
}

func (p *EventProcessor) ensureRequest(ctx context.Context, evt seckilldomain.Event) (*seckilldomain.Request, error) {
	req, err := p.requestRepo.FindByRequestNo(ctx, evt.RequestNo)
	if err == nil {
		return req, nil
	}
	if !errors.Is(err, seckilldomain.ErrRequestNotFound) {
		return nil, err
	}

	req = &seckilldomain.Request{
		RequestNo:  evt.RequestNo,
		ActivityID: evt.ActivityID,
		UserID:     evt.UserID,
		Status:     seckilldomain.RequestStatusProcessing,
	}
	if err = p.requestRepo.Create(ctx, req); err == nil {
		return req, nil
	}
	if !errors.Is(err, seckilldomain.ErrDuplicateSeckill) {
		return nil, err
	}
	return p.requestRepo.FindByRequestNo(ctx, evt.RequestNo)
}

func (p *EventProcessor) processProcessing(ctx context.Context, evt seckilldomain.Event) error {
	req, err := p.requestRepo.AdvanceProcessing(ctx, evt)
	if err != nil {
		return err
	}

	switch req.Status {
	case seckilldomain.RequestStatusOrderCreating:
		return p.processOrderCreating(ctx, evt)
	case seckilldomain.RequestStatusFailed:
		return p.compensateAndClear(ctx, evt.ActivityID, evt.UserID, evt.RequestNo, seckilldomain.Result{
			RequestNo:  evt.RequestNo,
			Status:     seckilldomain.RequestStatusFailed,
			FailReason: req.FailReason,
		})
	case seckilldomain.RequestStatusSuccess:
		return p.cache.SetResult(ctx, seckilldomain.Result{
			RequestNo: evt.RequestNo,
			Status:    seckilldomain.RequestStatusSuccess,
			OrderID:   req.OrderID,
		})
	default:
		return errors.New("unexpected request status after processing transition")
	}
}

func (p *EventProcessor) processOrderCreating(ctx context.Context, evt seckilldomain.Event) error {
	orderID, ok := seckilldomain.OrderIDFromRequestNo(evt.RequestNo)
	if !ok {
		return fmt.Errorf("invalid request_no for order creation: %s", evt.RequestNo)
	}

	_, err := p.orderClient.CreateOrder(ctx, &orderv1.CreateOrderReq{
		OrderId:    orderID,
		UserId:     evt.UserID,
		Currency:   "CNY",
		OrderKind:  orderdomain.OrderKindSeckill,
		ActivityId: evt.ActivityID,
		Items: []*orderv1.OrderItem{{
			ProductId:        evt.ProductID,
			SkuId:            evt.SKUID,
			Quantity:         1,
			SnapshotPrice:    evt.SeckillPrice,
			SnapshotCurrency: "CNY",
			ConvertedPrice:   evt.SeckillPrice,
		}},
	})
	if err != nil {
		if isExplicitCreateOrderFailure(err) {
			return p.rollbackOrderCreating(ctx, evt, seckilldomain.FailReasonCreateOrderFail)
		}
		return err
	}

	req, err := p.requestRepo.CompleteOrderCreating(ctx, evt)
	if err != nil {
		return err
	}
	return p.cache.SetResult(ctx, seckilldomain.Result{
		RequestNo: evt.RequestNo,
		Status:    seckilldomain.RequestStatusSuccess,
		OrderID:   orderIDOrRequestNo(req),
	})
}

func (p *EventProcessor) lookupOrder(ctx context.Context, evt seckilldomain.Event) (int64, bool, error) {
	orderID, ok := seckilldomain.OrderIDFromRequestNo(evt.RequestNo)
	if !ok {
		return 0, false, fmt.Errorf("invalid request_no for order lookup: %s", evt.RequestNo)
	}

	resp, err := p.orderClient.GetOrder(ctx, &orderv1.GetOrderReq{OrderId: orderID})
	if err != nil {
		if isOrderNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	order := resp.GetOrder()
	if order == nil {
		return 0, false, nil
	}
	if order.GetUserId() != evt.UserID || order.GetActivityId() != evt.ActivityID || order.GetOrderKind() != orderdomain.OrderKindSeckill {
		return 0, false, nil
	}
	return order.GetOrderId(), true, nil
}

func (p *EventProcessor) rollbackOrderCreating(ctx context.Context, evt seckilldomain.Event, reason string) error {
	req, err := p.requestRepo.RollbackOrderCreating(ctx, evt, reason)
	if err != nil {
		return err
	}
	return p.compensateAndClear(ctx, evt.ActivityID, evt.UserID, evt.RequestNo, seckilldomain.Result{
		RequestNo:  evt.RequestNo,
		Status:     seckilldomain.RequestStatusFailed,
		FailReason: req.FailReason,
	})
}

func (p *EventProcessor) compensateAndClear(ctx context.Context, activityID, userID int64, requestNo string, result seckilldomain.Result) error {
	if err := p.cache.Compensate(ctx, activityID, userID, requestNo, result); err != nil {
		return err
	}
	// 这里代表库存已经回补，本机售罄标记也要一起清掉，避免继续误杀。
	p.soldOut.Clear(activityID)
	p.logger.Info("compensation finished, clear local sold-out flag",
		logger.String("requestNo", requestNo),
		logger.Int64("activityID", activityID),
		logger.Int64("userID", userID),
		logger.String("status", result.Status),
		logger.String("failReason", result.FailReason))
	return nil
}

func orderIDOrRequestNo(req *seckilldomain.Request) int64 {
	if req.OrderID != 0 {
		return req.OrderID
	}
	orderID, _ := seckilldomain.OrderIDFromRequestNo(req.RequestNo)
	return orderID
}

func isExplicitCreateOrderFailure(err error) bool {
	bizErr, ok := kerrors.FromBizStatusError(err)
	if ok && orderdomain.IsCreateOrderBizStatus(bizErr.BizStatusCode()) {
		return true
	}
	return false
}

func isOrderNotFound(err error) bool {
	bizErr, ok := kerrors.FromBizStatusError(err)
	return ok && orderdomain.IsGetOrderNotFoundBizStatus(bizErr.BizStatusCode())
}
