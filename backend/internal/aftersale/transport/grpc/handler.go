package grpc

import (
	"context"
	"encoding/json"

	"github.com/XDWow/DouyinMall/backend/internal/aftersale/domain"
	"github.com/XDWow/DouyinMall/backend/internal/aftersale/usecase"
	aftersalev1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/aftersale/v1"
)

type Handler struct {
	createRequestUC *usecase.CreateAfterSaleRequestUseCase
	getRequestUC    *usecase.GetAfterSaleRequestUseCase
}

func NewHandler(createRequestUC *usecase.CreateAfterSaleRequestUseCase, getRequestUC *usecase.GetAfterSaleRequestUseCase) *Handler {
	return &Handler{
		createRequestUC: createRequestUC,
		getRequestUC:    getRequestUC,
	}
}

func (h *Handler) CreateAfterSaleRequest(ctx context.Context, req *aftersalev1.CreateAfterSaleRequestReq) (*aftersalev1.CreateAfterSaleRequestResp, error) {
	var metadata map[string]any
	if payload := req.GetMetadataJson(); payload != "" {
		_ = json.Unmarshal([]byte(payload), &metadata)
	}

	result, err := h.createRequestUC.Execute(ctx, usecase.CreateAfterSaleRequestCmd{
		UserID:      req.GetUserId(),
		OrderID:     req.GetOrderId(),
		ItemID:      req.GetItemId(),
		RequestType: requestTypeToString(req.GetRequestType()),
		Reason:      req.GetReason(),
		SessionID:   req.GetSessionId(),
		TraceID:     req.GetTraceId(),
		Metadata:    metadata,
	})
	if err != nil {
		return nil, err
	}

	return &aftersalev1.CreateAfterSaleRequestResp{
		Request: toProtoRequest(result),
	}, nil
}

func (h *Handler) GetAfterSaleRequest(ctx context.Context, req *aftersalev1.GetAfterSaleRequestReq) (*aftersalev1.GetAfterSaleRequestResp, error) {
	result, err := h.getRequestUC.Execute(ctx, usecase.GetAfterSaleRequestCmd{
		RequestNo: req.GetRequestNo(),
	})
	if err != nil {
		return nil, err
	}
	return &aftersalev1.GetAfterSaleRequestResp{
		Request: toProtoRequest(result),
	}, nil
}

func toProtoRequest(item *domain.Request) *aftersalev1.AfterSaleRequest {
	if item == nil {
		return nil
	}
	return &aftersalev1.AfterSaleRequest{
		RequestNo:   item.RequestNo,
		UserId:      item.UserID,
		OrderId:     item.OrderID,
		ItemId:      item.ItemID,
		RequestType: stringToRequestType(string(item.RequestType)),
		Reason:      item.Reason,
		Status:      stringToStatus(string(item.Status)),
		CreatedAt:   item.CreatedAt.Unix(),
	}
}

func requestTypeToString(value aftersalev1.AfterSaleRequestType) string {
	switch value {
	case aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_EXCHANGE:
		return string(domain.RequestTypeExchange)
	default:
		return string(domain.RequestTypeReturn)
	}
}

func stringToRequestType(value string) aftersalev1.AfterSaleRequestType {
	switch value {
	case string(domain.RequestTypeExchange):
		return aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_EXCHANGE
	default:
		return aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_RETURN
	}
}

func stringToStatus(value string) aftersalev1.AfterSaleRequestStatus {
	switch value {
	case string(domain.StatusPendingReview):
		return aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_PENDING_REVIEW
	case string(domain.StatusApproved):
		return aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_APPROVED
	case string(domain.StatusRejected):
		return aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_REJECTED
	case string(domain.StatusCanceled):
		return aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_CANCELED
	case string(domain.StatusCompleted):
		return aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_COMPLETED
	default:
		return aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_UNKNOWN
	}
}


