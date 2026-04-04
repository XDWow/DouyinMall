package grpc

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/usecase"
	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
)

type Handler struct {
	createActivityUC *usecase.CreateActivityUseCase
	updateStatusUC   *usecase.UpdateActivityStatusUseCase
	getActivityUC    *usecase.GetActivityUseCase
	submitUC         *usecase.SubmitUseCase
	getResultUC      *usecase.GetResultUseCase
}

func NewHandler(
	createActivityUC *usecase.CreateActivityUseCase,
	updateStatusUC *usecase.UpdateActivityStatusUseCase,
	getActivityUC *usecase.GetActivityUseCase,
	submitUC *usecase.SubmitUseCase,
	getResultUC *usecase.GetResultUseCase,
) *Handler {
	return &Handler{
		createActivityUC: createActivityUC,
		updateStatusUC:   updateStatusUC,
		getActivityUC:    getActivityUC,
		submitUC:         submitUC,
		getResultUC:      getResultUC,
	}
}

func (h *Handler) CreateActivity(ctx context.Context, req *seckillv1.CreateActivityReq) (*seckillv1.CreateActivityResp, error) {
	id, err := h.createActivityUC.Execute(ctx, usecase.CreateActivityCmd{
		ActivityName: req.GetActivityName(),
		ProductID:    req.GetProductId(),
		SKUID:        req.GetSkuId(),
		SeckillPrice: req.GetSeckillPrice(),
		TotalStock:   req.GetTotalStock(),
		StartTime:    time.Unix(req.GetStartTime(), 0),
		EndTime:      time.Unix(req.GetEndTime(), 0),
		Status:       req.GetStatus(),
		LimitPerUser: req.GetLimitPerUser(),
	})
	if err != nil {
		return nil, err
	}
	return &seckillv1.CreateActivityResp{ActivityId: id}, nil
}

func (h *Handler) UpdateActivityStatus(ctx context.Context, req *seckillv1.UpdateActivityStatusReq) (*seckillv1.UpdateActivityStatusResp, error) {
	if err := h.updateStatusUC.Execute(ctx, req.GetActivityId(), req.GetStatus()); err != nil {
		return nil, err
	}
	return &seckillv1.UpdateActivityStatusResp{}, nil
}

func (h *Handler) GetActivity(ctx context.Context, req *seckillv1.GetActivityReq) (*seckillv1.GetActivityResp, error) {
	activity, err := h.getActivityUC.Execute(ctx, req.GetActivityId())
	if err != nil {
		return nil, err
	}
	return &seckillv1.GetActivityResp{Activity: &seckillv1.SeckillActivity{
		Id:             activity.ID,
		ActivityName:   activity.ActivityName,
		ProductId:      activity.ProductID,
		SkuId:          activity.SKUID,
		SeckillPrice:   activity.SeckillPrice,
		TotalStock:     activity.TotalStock,
		AvailableStock: activity.AvailableStock,
		StartTime:      activity.StartTime.Unix(),
		EndTime:        activity.EndTime.Unix(),
		Status:         activity.Status,
		LimitPerUser:   activity.LimitPerUser,
	}}, nil
}

func (h *Handler) SubmitSeckill(ctx context.Context, req *seckillv1.SubmitSeckillReq) (*seckillv1.SubmitSeckillResp, error) {
	result, err := h.submitUC.Execute(ctx, usecase.SubmitCmd{ActivityID: req.GetActivityId(), UserID: req.GetUserId()})
	if err != nil && result == nil {
		return nil, err
	}
	return &seckillv1.SubmitSeckillResp{
		RequestNo: result.RequestNo,
		Status:    result.Status,
		Message:   result.FailReason,
	}, err
}

func (h *Handler) GetSeckillResult(ctx context.Context, req *seckillv1.GetSeckillResultReq) (*seckillv1.GetSeckillResultResp, error) {
	result, err := h.getResultUC.Execute(ctx, req.GetRequestNo())
	if err != nil {
		return nil, err
	}
	return &seckillv1.GetSeckillResultResp{
		RequestNo:  result.RequestNo,
		Status:     result.Status,
		OrderId:    result.OrderID,
		FailReason: result.FailReason,
	}, nil
}


