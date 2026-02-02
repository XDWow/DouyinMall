package grpc

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	paymentv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/payment/v1"
	"time"
)

type PaymentHandler struct {
	prePayUC *usecase.NativePrePaymentUC
	GetPayUC *usecase.GetPaymentUC
}

func NewPaymentHandler(
	prePayUC *usecase.NativePrePaymentUC,
	getPayUC *usecase.GetPaymentUC,
) *PaymentHandler {
	return &PaymentHandler{
		prePayUC: prePayUC,
		GetPayUC: getPayUC,
	}
}

func (h *PaymentHandler) NativePrepay(ctx context.Context, req *paymentv1.NativePrePayRequest) (res *paymentv1.NativePrePayResponse, err error) {
	cmd := usecase.PrePaymentCmd{
		Amt: domain.Amount{
			Total:    req.GetAmt().Total,
			Currency: req.GetAmt().Currency,
		},
		BizTradeNo:  req.GetBizTradeNo(),
		Description: req.GetDescription(),
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*5) // 给时间，给你5s，usecase内部自己决定怎么花时间
	defer cancel()
	codeUrl, err := h.prePayUC.Execute(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return &paymentv1.NativePrePayResponse{CodeUrl: codeUrl}, nil
}

func (h *PaymentHandler) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (res *paymentv1.GetPaymentResponse, err error) {
	if req.GetBizTradeNo() == "" {
		return nil, errors.New("biz_trade_no is empty")
	}
	cmd := usecase.QueryPaymentCmd{
		BizTradeNo: req.GetBizTradeNo(),
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	pmt, err := h.GetPayUC.Execute(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return &paymentv1.GetPaymentResponse{
		Status: toProtoPaymentStatus(pmt.Status),
	}, nil
}

func toProtoPaymentStatus(s domain.PaymentStatus) paymentv1.PaymentStatus {
	switch s {
	case domain.PaymentStatusInit:
		return paymentv1.PaymentStatus_PaymentStatusInit
	case domain.PaymentStatusSuccess:
		return paymentv1.PaymentStatus_PaymentStatusSuccess
	case domain.PaymentStatusFailed:
		return paymentv1.PaymentStatus_PaymentStatusFailed
	case domain.PaymentStatusRefund:
		return paymentv1.PaymentStatus_PaymentStatusRefund
	default:
		return paymentv1.PaymentStatus_PaymentStatusUnknown
	}
}
